package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func usage() {
	fmt.Printf("Usage: %s <twitch-username>\n", os.Args[0])
	os.Exit(1)
}

func isOfflineMessage(output string) bool {
	lower := strings.ToLower(output)
	offlineHints := []string{
		"is offline",
		"not currently live",
		"channel is not live",
		"channel is offline",
	}

	for _, hint := range offlineHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}

	return false
}

func parseFormatCodes(output string) []string {
	formatSet := map[string]struct{}{}
	lines := strings.Split(output, "\n")
	regex := regexp.MustCompile(`^[0-9]{3,4}p(?:[0-9]{2})?$`)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		code := fields[0]
		if regex.MatchString(code) {
			formatSet[code] = struct{}{}
		}
	}

	formatCodes := make([]string, 0, len(formatSet))
	for code := range formatSet {
		formatCodes = append(formatCodes, code)
	}
	sort.Strings(formatCodes)
	return formatCodes
}

func formats(name string) ([]string, error) {
	cmd := exec.Command("yt-dlp", "-F", "https://twitch.tv/"+name)
	output, err := cmd.CombinedOutput()
	outputText := strings.TrimSpace(string(output))

	if err != nil {
		if isOfflineMessage(outputText) {
			return nil, fmt.Errorf("stream '%s' is offline", name)
		}

		if outputText != "" {
			return nil, fmt.Errorf("yt-dlp failed: %w\n%s", err, outputText)
		}
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	formatCodes := parseFormatCodes(outputText)
	if len(formatCodes) == 0 {
		return nil, fmt.Errorf("yt-dlp returned no compatible quality formats for '%s'", name)
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

func getFmt(name string) string {
	fmts, err := formats(name)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Pick a format:")
	return pickOne(fmts)
}

func playStream(fmt, name string) {
	url := "https://twitch.tv/" + name

	cmd := exec.Command("mpv", "--really-quiet", "--title=Twitch", "--ytdl-format="+fmt, url)

	// stderr to /dev/null
	cmd.Stderr = nil

	// run asynchronously
	err := cmd.Start()
	if err != nil {
		log.Fatalf("[ERROR] %v: ", err)
	}

	// choto-mate ne! ... give me back my terminal
	err = cmd.Process.Release()
	if err != nil {
		log.Fatalf("[ERROR] %v: ", err)
	}
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	name := os.Args[1]
	fmtStr := getFmt(name)

	playStream(fmtStr, name)
}
