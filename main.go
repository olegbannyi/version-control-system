package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	helpCmd    = "--help"
	vcsDir     = "./vcs"
	commitsDir = "./vcs/commits"
)

var commands []Cmd

type Cmd struct {
	name        string
	description string
	handler     func()
}

func main() {
	commands = []Cmd{
		{"--help", "", helpHandler},
		{"config", "Get and set a username.", configHandler},
		{"add", "Add a file to the index.", addHandler},
		{"log", "Show commit logs.", logHandler},
		{"commit", "Save changes.", commitHandler},
		{"checkout", "Restore a file.", checkoutHandler},
	}

	err := os.MkdirAll(vcsDir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll(commitsDir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	cmd := helpCmd
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	getCommand(cmd).handler()
}

func helpHandler() {
	fmt.Println("These are SVCS commands:")
	for _, cmd := range commands {
		if cmd.name == helpCmd {
			continue
		}
		fmt.Printf("%-10s %s\n", cmd.name, cmd.description)
	}
}

func getCommand(name string) Cmd {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd
		}
	}

	return Cmd{name: "", description: "", handler: func() {
		fmt.Printf("'%s' is not a SVCS command.\n", name)
	}}
}

func configHandler() {
	filename := filepath.Join(vcsDir, "config.txt")
	if len(os.Args) > 2 {
		err := os.WriteFile(filename, []byte(os.Args[2]), 0644)
		if err != nil {
			log.Println(err)
		}
		fmt.Printf("The username is %s.\n", os.Args[2])
		return
	}
	name := getUserName()
	if len(name) == 0 {
		fmt.Println("Please, tell me who you are.")
	} else {
		fmt.Printf("The username is %s.\n", name)
	}
}

func getUserName() string {
	filename := filepath.Join(vcsDir, "config.txt")
	name, err := os.ReadFile(filename)

	if os.IsNotExist(err) {
		return ""
	} else if err != nil {
		log.Fatalln(err)
	} else if len(name) == 0 {
		return ""
	}

	return string(name)
}

func addHandler() {
	filename := filepath.Join(vcsDir, "index.txt")
	if len(os.Args) > 2 {
		trackedFile := filepath.Join(".", os.Args[2])
		_, err := os.Stat(trackedFile)
		if os.IsNotExist(err) {
			fmt.Printf("Can't find '%s'.\n", os.Args[2])
		} else if err != nil {
			log.Fatalln(err)
		} else if !isFileVersioned(os.Args[2]) {
			f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatalln(err)
			}
			defer f.Close()
			if _, err := f.Write([]byte(os.Args[2] + "\n")); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("The file '%s' is tracked.\n", os.Args[2])
		}
	} else {
		trackedFiles := listTrackedFiles()
		if len(trackedFiles) == 0 {
			fmt.Println("Add a file to the index.")
		} else {
			fmt.Println("Tracked files:")
			for _, f := range trackedFiles {
				fmt.Println(f)
			}
		}
	}
}

func logHandler() {

	if logRecords := listLogs(); len(logRecords) == 0 {
		fmt.Println("No commits yet.")
	} else {
		slices.Reverse(logRecords)
		for _, record := range logRecords {
			fmt.Println(record)
		}
	}
}

func commitHandler() {
	if len(os.Args) > 2 {
		hash := trackedFilesHash()
		if hash == "" {
			fmt.Println("Nothing to commit.")
			return
		}

		commitDir := filepath.Join(commitsDir, hash)
		_, err := os.Stat(commitDir)

		if os.IsNotExist(err) {
			err := os.MkdirAll(commitDir, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
			for _, file := range listTrackedFiles() {
				if err := copyFile(filepath.Join(commitDir, file), file); err != nil {
					log.Fatalln(err)
				}
			}
			writeLog(hash, os.Args[2])
			fmt.Println("Changes are committed.")
		} else if err != nil {
			log.Fatalln(err)
		} else {
			fmt.Println("Nothing to commit.")
		}
	} else {
		fmt.Println("Message was not passed.")
	}
}

func checkoutHandler() {
	if len(os.Args) > 2 {
		commitDir := filepath.Join(commitsDir, os.Args[2])
		_, err := os.Stat(commitDir)
		if os.IsNotExist(err) {
			fmt.Println("Commit does not exist.")
		} else if err != nil {
			log.Fatalln(err)
		} else {
			files, err := os.ReadDir(commitDir)
			if err != nil {
				log.Fatalln(err)
			}

			for _, file := range files {
				copyFile(file.Name(), filepath.Join(commitDir, file.Name()))
			}

			fmt.Printf("Switched to commit %s.\n", os.Args[2])
		}
	} else {
		fmt.Println("Commit id was not passed.")
	}
}

func isFileVersioned(file string) bool {
	for _, line := range listTrackedFiles() {
		if line == file {
			return true
		}
	}

	return false
}

func listTrackedFiles() []string {
	filename := filepath.Join(vcsDir, "index.txt")
	content, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return []string{}
	}

	if err != nil {
		log.Fatalln(err)
	}

	list := strings.Split(string(content), "\n")
	if len(list) == 0 {
		return list
	}

	return list[:len(list)-1]
}

func listLogs() []string {
	filename := filepath.Join(vcsDir, "log.txt")
	content, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return []string{}
	}

	if err != nil {
		log.Fatalln(err)
	}

	list := strings.Split(string(content), "\n")
	if len(list) == 0 {
		return list
	}

	return list[:len(list)-1]
}

func trackedFilesHash() string {
	files := listTrackedFiles()

	if len(files) == 0 {
		return ""
	}

	var hashes strings.Builder
	for _, f := range files {
		_, err := hashes.WriteString(fileHash(f))
		if err != nil {
			log.Fatalln(err)
		}
	}

	h := md5.New()
	io.WriteString(h, hashes.String())

	return fmt.Sprintf("%x", h.Sum(nil))
}

func fileHash(file string) string {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func copyFile(destinationFile, sourceFile string) error {
	source, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func writeLog(hash, message string) {
	name := getUserName()
	if len(name) == 0 {
		fmt.Println("Please, tell me who you are.")
		return
	}

	filename := filepath.Join(vcsDir, "log.txt")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()
	record := message + "\nAuthor: " + name + "\ncommit " + hash + "\n\n"
	if _, err := f.WriteString(record); err != nil {
		log.Fatal(err)
	}
}
