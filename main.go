package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func usage() {
	fmt.Printf("Usage: %s <twitch-username>\n", os.Args[0])
	os.Exit(1)
}

func formats(name string) []string {
	cmd := exec.Command("youtube-dl", "-F", "https://twitch.tv/"+name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error occurred: %s", err)
	}

	formatCodes := []string{}
	lines := strings.Split(string(output), "\n")
	regex := regexp.MustCompile(`^\d+p(\S+)?`)

	for _, line := range lines {
		if matches := regex.FindStringSubmatch(line); matches != nil {
			formatCodes = append(formatCodes, matches[0])
		}
	}

	return formatCodes
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
			log.Fatal(err)
		}

		if index, err := strconv.Atoi(answer); err == nil && index >= 1 && index <= len(lst) {
			return lst[index-1]
		}

		fmt.Println("Invalid input. Please enter a number between 1 and", len(lst))
	}
}

func getFmt(name string) string {
	fmts := formats(name)
	if len(fmts) == 0 {
		log.Fatal("No available format found for the stream.")
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
		log.Fatal(err)
	}

	// choto-mate ne! ... give me back my terminal
	err = cmd.Process.Release()
	if err != nil {
		log.Fatal(err)
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
