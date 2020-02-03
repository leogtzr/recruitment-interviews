package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func sanitizeUserInput(input string) string {
	return strings.TrimSpace(input)
}

// Transforms user's input to a Command
func userInputToCmd(input string) Command {
	fullCommand := words(input)
	input = fullCommand[0]
	input = sanitizeUserInput(input)
	input = strings.ToLower(input)
	switch input {
	case "exit", "quit", ":q", "/q", "q":
		return exitCmd
	case "topics", "tps", "t", "/t", ":t":
		return topicsCmd
	case "help", ":h", "/h", "--h", "-h":
		return helpCmd
	case "use", "u", "/u", ":u", "-u", "--u", "set":
		return useCmd
	}
	return noCmd
}

func dirExists(dirPath string) bool {
	if _, err := os.Stat(dirPath); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return true
}

func listTopics(interviewsDir string) {
	topicsDir := filepath.Join(interviewsDir, "topics")

	topicsInDir := []string{}

	if !dirExists(topicsDir) {
		log.Fatalf("'%s' does not exist", topicsDir)
	}

	err := filepath.Walk(topicsDir, func(path string, info os.FileInfo, err error) error {
		path = filepath.Base(path)
		if path == "topics" {
			return nil
		}
		topicsInDir = append(topicsInDir, path)
		return nil
	})
	if err != nil {
		panic(err)
	}

	for _, topic := range topicsInDir {
		fmt.Println(topic)
	}

}

// TODO: ...
func printHelp() {
	usage := `
commands:


	`

	fmt.Println(usage)
}

func words(input string) []string {
	return strings.Fields(input)
}
