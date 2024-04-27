package main

import (
	"Project2Demo/FileSystem"
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	// this is I think the test I promised you - except that maybe the string is too short
	FileSystem.InitializeFileSystem()
	scanner := bufio.NewScanner(os.Stdin)

	//fmt.Println(FileSystem.Read(newFileInode))
	for {
		fmt.Printf("Please enter a command: ")
		scanner.Scan()
		input := scanner.Text()
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "exit" {
			fmt.Println("Exiting shell.")
			return
		}

		args := strings.Fields(input)
		if len(args) == 0 {
			fmt.Println("Please enter a command.")
			continue
		}

		command := args[0]
		commandArgs := args[1:]

		fmt.Printf("Please enter a command:") // Printing Prompt
		if _, err := fmt.Scanln(&command); err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		if command == "exit" { //If user input is the same as the string
			fmt.Println("Exiting shell.") //Print Statement
			return                        //Returning
		}

		switch command {
		case "mv":
			if len(commandArgs) < 2 {
				fmt.Println("Usage: mv <source> <destination>")
			} else {
				moveFile(commandArgs[0], commandArgs[1])
			}

		case "mkdir":
			if len(commandArgs) < 1 {
				fmt.Println("Usage: mkdir <directory name>")
			} else {
				makeDirectory(commandArgs[0])
			}

		case "cp":
			if len(commandArgs) < 2 {
				fmt.Println("Usage: cp <source> <destination>")
			} else {
				copyFile(commandArgs[0], commandArgs[1])
			}

		case "more":
			if len(commandArgs) < 1 {
				fmt.Println("Usage: more <file name>")
			} else {
				displayFileContent(commandArgs[0])
			}

		case "rm":
			if len(commandArgs) < 1 {
				fmt.Println("Usage: rm <file name>")
			} else {
				removeFile(commandArgs[0])
			}

		case ">>":
			if len(commandArgs) < 2 {
				fmt.Println("Usage: >> <file name> <content>")
			} else {
				appendToFile(commandArgs[0], strings.Join(commandArgs[1:], " "))
			}

		default:
			fmt.Println("Command not supported.")
		}
	}

}

func moveFile(source, destination string) {
	// Get inodes for source and destination paths
	parentSourceInode, sourceInode, _, sourceInodeNum, err := FileSystem.GetParentAndChildInodes(source)
	if err != nil {
		fmt.Println("Error getting source inodes:", err)
		return
	}
	parentDestInode, _, _, destInodeNum, err := FileSystem.GetParentAndChildInodes(destination)
	if err != nil {
		fmt.Println("Error getting destination inodes:", err)
		return
	}

	// Read content from the source file
	content := FileSystem.Read(&sourceInode)
	if content == "" {
		fmt.Println("Error reading content from source file.")
		return
	}

	// Write content to the destination
	FileSystem.Write(&parentDestInode, destInodeNum, []byte(content))

	// Unlink the source file
	FileSystem.Unlink(sourceInodeNum, parentSourceInode)

	fmt.Printf("Moved from %s to %s.\n", source, destination)
}

func makeDirectory(directoryName string) {
	parentInode, _, parentInodeNum, _, err := FileSystem.GetParentAndChildInodes(directoryName)
	if err != nil {
		fmt.Println("Error getting parent inode:", err)
		return
	}

	// Create directory and ignore the returned newDirInode if not needed
	_, _ = FileSystem.CreateDirectoryFile(parentInodeNum, 0) // Assuming 0 is a placeholder
	FileSystem.Write(&parentInode, parentInodeNum, []byte{}) // This just calls Write without error check

	fmt.Printf("Directory %s created.\n", directoryName)
}

func copyFile(source, destination string) {
	_, srcInode, _, _, err := FileSystem.GetParentAndChildInodes(source)
	if err != nil {
		fmt.Println("Error reading source file:", err)
		return
	}
	_, destParentInode, _, destInodeNum, err := FileSystem.GetParentAndChildInodes(destination)
	if err != nil {
		fmt.Println("Error preparing destination:", err)
		return
	}

	// Read content from the source
	content := FileSystem.Read(&srcInode)
	if content == "" {
		fmt.Println("Error reading content from source file.")
		return
	}

	// Write content to the destination
	FileSystem.Write(&destParentInode, destInodeNum, []byte(content))

	fmt.Printf("Copied from %s to %s.\n", source, destination)
}

func displayFileContent(fileName string) {
	_, fileInode, _, _, err := FileSystem.GetParentAndChildInodes(fileName)
	if err != nil {
		fmt.Println("Error accessing file:", err)
		return
	}
	content := FileSystem.Read(&fileInode) // Assuming Read returns string
	fmt.Println(content)
}

func removeFile(fileName string) {
	_, _, _, inodeNum, err := FileSystem.GetParentAndChildInodes(fileName)
	if err != nil {
		fmt.Println("Error finding file to remove:", err)
		return
	}
	FileSystem.Unlink(inodeNum, FileSystem.RootFolder) // Assuming Unlink function exists
	fmt.Printf("File %s removed.\n", fileName)
}

func appendToFile(fileName, content string) {
	_, fileInode, inodeNum, _, err := FileSystem.GetParentAndChildInodes(fileName)
	if err != nil {
		fmt.Println("Error opening file to append:", err)
		return
	}
	existingContent := FileSystem.Read(&fileInode) // Assuming Read returns string
	updatedContent := existingContent + content
	FileSystem.Write(&fileInode, inodeNum, []byte(updatedContent)) // Assuming Write takes bytes
	fmt.Printf("Appended to %s.\n", fileName)
}

/*
build a version of
• mkdir
• mv
• cp
• rm
• more
• and the redirect operator >>
◦ to be able to redirect text into one of your virtual files
That will use your virtual file system
• let all of the other command continue to use the existing file system
• for example if I do
◦ cat really_long_file.txt >> test.txt
◦ mkdir temp
◦ mv text.txt temp




*/
