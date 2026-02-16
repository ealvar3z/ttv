package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type formatEntry struct {
	FormatID string `json:"format_id"`
}

type ytDLPMetadata struct {
	Formats []formatEntry `json:"formats"`
}

var qualityIDRegex = regexp.MustCompile(`^[0-9]{3,4}p(?:[0-9]{2})?$`)
var qualityPartsRegex = regexp.MustCompile(`^([0-9]{3,4})p([0-9]{2})?$`)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [-v] <twitch-username>\n", os.Args[0])
	os.Exit(1)
}

func parseArgs(args []string) (string, bool) {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	verbose := flags.Bool("v", false, "print full yt-dlp diagnostics on failure")
	flags.BoolVar(verbose, "verbose", false, "print full yt-dlp diagnostics on failure")

	if err := flags.Parse(args); err != nil {
		usage()
	}

	remaining := flags.Args()
	if len(remaining) != 1 {
		usage()
	}

	return remaining[0], *verbose
}

func matchAny(text string, hints []string) bool {
	for _, hint := range hints {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}

func classifyYTDLPFailure(runErr error, output string) string {
	lower := strings.ToLower(strings.TrimSpace(output))
	errText := ""
	if runErr != nil {
		errText = strings.ToLower(runErr.Error())
	}
	combined := lower + "\n" + errText

	switch {
	case matchAny(combined, []string{
		"is offline",
		"not currently live",
		"channel is offline",
		"channel is not live",
	}):
		return "stream is offline"
	case matchAny(combined, []string{
		"geo-restricted",
		"geo restricted",
		"not available in your country",
		"unavailable in your country",
		"from your location",
	}):
		return "stream is geo-restricted from your location"
	case matchAny(combined, []string{
		"login required",
		"authentication required",
		"sign in",
		"cookies are needed",
		"cookies",
		"age-restricted",
		"members-only",
		"subscriber-only",
	}):
		return "stream requires authentication (cookies/login)"
	case matchAny(combined, []string{
		"private",
		"forbidden",
		"http error 403",
		"access denied",
	}):
		return "stream is private or access is denied"
	case matchAny(combined, []string{
		"timed out",
		"temporary failure in name resolution",
		"network is unreachable",
		"connection refused",
		"unable to download webpage",
		"http error 429",
		"rate limit",
		"http error 5",
	}):
		return "network/server issue while contacting Twitch/yt-dlp"
	case matchAny(combined, []string{
		"executable file not found",
		"no such file or directory",
	}):
		return "yt-dlp is not installed or not in PATH"
	default:
		return "yt-dlp returned an unknown error"
	}
}

func parseQualityParts(id string) (height int, fps int, ok bool) {
	m := qualityPartsRegex.FindStringSubmatch(id)
	if m == nil {
		return 0, 0, false
	}

	height, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, false
	}

	if m[2] == "" {
		return height, 0, true
	}

	fps, err = strconv.Atoi(m[2])
	if err != nil {
		return 0, 0, false
	}

	return height, fps, true
}

func sortFormatsByQuality(formatCodes []string) {
	sort.Slice(formatCodes, func(i, j int) bool {
		hi, fi, _ := parseQualityParts(formatCodes[i])
		hj, fj, _ := parseQualityParts(formatCodes[j])

		if hi != hj {
			return hi > hj
		}
		if fi != fj {
			return fi > fj
		}
		return formatCodes[i] > formatCodes[j]
	})
}

func parseFormatCodes(metadataJSON string) ([]string, error) {
	var metadata ytDLPMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, err
	}

	formatSet := map[string]struct{}{}
	for _, format := range metadata.Formats {
		if qualityIDRegex.MatchString(format.FormatID) {
			formatSet[format.FormatID] = struct{}{}
		}
	}

	formatCodes := make([]string, 0, len(formatSet))
	for code := range formatSet {
		formatCodes = append(formatCodes, code)
	}
	sortFormatsByQuality(formatCodes)
	return formatCodes, nil
}

func formatYTDLPError(name string, runErr error, output string, verbose bool) error {
	cause := classifyYTDLPFailure(runErr, output)
	base := fmt.Sprintf("could not fetch formats for '%s': %s", name, cause)

	if verbose && strings.TrimSpace(output) != "" {
		return fmt.Errorf("%s\n\nyt-dlp output:\n%s", base, strings.TrimSpace(output))
	}
	if strings.TrimSpace(output) != "" {
		return fmt.Errorf("%s (rerun with -v for full yt-dlp output)", base)
	}
	if runErr != nil {
		return fmt.Errorf("%s: %w", base, runErr)
	}
	return errors.New(base)
}

func formats(name string, verbose bool) ([]string, error) {
	cmd := exec.Command("yt-dlp", "-J", "https://twitch.tv/"+name)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	stdoutText := strings.TrimSpace(stdout.String())
	stderrText := strings.TrimSpace(stderr.String())

	if runErr != nil {
		output := stderrText
		if output == "" {
			output = stdoutText
		}
		return nil, formatYTDLPError(name, runErr, output, verbose)
	}

	formatCodes, err := parseFormatCodes(stdoutText)
	if err != nil {
		diagnostics := stderrText
		if diagnostics == "" {
			diagnostics = stdoutText
		}
		if verbose && diagnostics != "" {
			return nil, fmt.Errorf("yt-dlp returned invalid JSON for '%s': %w\n\nraw output:\n%s", name, err, diagnostics)
		}
		return nil, fmt.Errorf("yt-dlp returned invalid JSON for '%s': %w", name, err)
	}

	if len(formatCodes) == 0 {
		return nil, fmt.Errorf("no compatible Twitch quality formats found for '%s'", name)
	}

	return formatCodes, nil
}

func pickOne(lst []string) string {
	for i, v := range lst {
		fmt.Printf("%d. %s\n", i+1, v)
	}

	var answer string
	for {
		fmt.Print("? ")
		_, err := fmt.Scanln(&answer)
		if err != nil {
			log.Fatalf("[ERROR] %v: ", err)
		}

		if index, err := strconv.Atoi(answer); err == nil && index >= 1 && index <= len(lst) {
			return lst[index-1]
		}

		fmt.Println("Invalid input. Please enter a number between 1 and", len(lst))
	}
}

func getFmt(name string, verbose bool) string {
	fmts, err := formats(name, verbose)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Pick a format:")
	return pickOne(fmts)
}

func playStream(fmtCode, name string) {
	url := "https://twitch.tv/" + name

	cmd := exec.Command("mpv", "--really-quiet", "--title=Twitch", "--ytdl-format="+fmtCode, url)

	// run asynchronously
	err := cmd.Start()
	if err != nil {
		log.Fatalf("[ERROR] %v: ", err)
	}

	// Give terminal back immediately.
	err = cmd.Process.Release()
	if err != nil {
		log.Fatalf("[ERROR] %v: ", err)
	}
}

func main() {
	name, verbose := parseArgs(os.Args[1:])
	fmtCode := getFmt(name, verbose)

	playStream(fmtCode, name)
}
