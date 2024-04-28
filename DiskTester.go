package main
//PRogramming Project 3
//Lukas Siemers

import (
	"Project2Demo/FileSystem"
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	FileSystem.InitializeFileSystem()
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("Please enter a command: ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Println("Error reading input:", err)
			}
			break
		}

		input := scanner.Text()
		input = strings.TrimSpace(input)
		if input == "exit" {
			fmt.Println("Exiting shell.")
			break
		}

		args := strings.Fields(input)
		if len(args) == 0 {
			fmt.Println("Please enter a command.")
			continue
		}

		command := args[0]
		commandArgs := args[1:]

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
				moveFile(commandArgs[0], commandArgs[1])
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
				fmt.Println("Usage: <file name> >> <content>")
			} else {
				appendToFile(commandArgs[0], strings.Join(commandArgs[1:], " "))
			}
		default:
			osCommand(command, commandArgs)
			fmt.Println("All other commands")
		}
	}
}
func getParentandChildInodes(path string) (parentinode FileSystem.INode, childinode FileSystem.INode, parentinodenum int, childinodenum int) {
	stringSlice := strings.Split(path, "/")
	newDirectory := stringSlice[len(stringSlice)-1]
	stringSlice = stringSlice[:len(stringSlice)-1]
	var toPath string
	for _, dir := range stringSlice {
		if dir != "" {
			toPath = toPath + "/" + dir
		}
	}
	parentinode, parentinodenum = FileSystem.FindSubdirectories(toPath)
	childinode, childinodenum = FileSystem.Open(FileSystem.CREATE, newDirectory, parentinode)
	return parentinode, childinode, parentinodenum, childinodenum
}

func moveFile(source, destination string) {
	moveFile(source, destination)
	removeFile(destination)
}

func makeDirectory(directoryName string) {
	parentInode, childInode, parentInodeNum, childInodeNum := getParentandChildInodes(directoryName)
	if !parentInode.IsValid || !childInode.IsValid {
		fmt.Println("Error: Failed to get necessary inode information")
		return
	}

	directoryBlock, err := FileSystem.CreateDirectoryFile(parentInodeNum, childInodeNum)
	if !err.IsValid {
		fmt.Println("Error creating directory:", err)
		return
	}

	bytesForDirectoryBlock := FileSystem.EncodeToBytes(directoryBlock)

	FileSystem.Write(&childInode, childInodeNum, bytesForDirectoryBlock)
	fmt.Printf("Directory '%s' created successfully.\n", directoryName)
}

func moveFileContent(source, destination string) {
	_, movingInode, _, movingInodeStatus := getParentandChildInodes(source)
	if movingInodeStatus == -1 { // Assuming -1 indicates an error or invalid inode.
		fmt.Printf("Error retrieving source inode for %s or inode is not valid.\n", source)
		return
	}
	if !movingInode.IsValid { // Check if the inode is valid.
		fmt.Println("Source inode is not valid")
		return
	}

	// Read content from the source inode.
	fileContent := FileSystem.Read(&movingInode)

	// Retrieve inode information for the destination file, correctly capturing all return values.
	_, toInode, _, toInodeStatus := getParentandChildInodes(destination)
	if toInodeStatus == -1 { // Similarrly, -1 for error or invalid inode.
		fmt.Printf("Error retrieving destination inode for %s or inode is not valid.\n", destination)
		return
	}
	if !toInode.IsValid {
		fmt.Println("Destination inode is not valid")
		return
	}

	// Convert the read content to bytes.
	inputContent := []byte(fileContent)

	// Write the content to the destination inode.
	FileSystem.Write(&toInode, toInodeStatus, inputContent)

	fmt.Printf("Content moved successfully from %s to %s.\n", source, destination)
}

func displayFileContent(fileName string) {
	_, childInode, _, childInodeStatus := getParentandChildInodes(fileName)
	if childInodeStatus == -1 { // Assuming -1 indicates an error or invalid inode.
		fmt.Printf("Error retrieving inode for %s or inode is not valid.\n", fileName)
		return
	}
	if !childInode.IsValid { // Check if the inode is valid.
		fmt.Println("Inode for the specified file is not valid")
		return
	}

	// Check if the file has content to read by checking the pointer to the first direct blocke.
	if childInode.DirectBlock1 == 0 {
		fmt.Println("Nothing to read in the file.")
	} else {
		// Read content from the file.
		fileContent := FileSystem.Read(&childInode)
		if fileContent == "" {
			fmt.Println("File is empty.")
		} else {
			// Output the read content.
			fmt.Println("File content:")
			fmt.Println(fileContent)
		}
	}
}

func removeFile(fileName string) {
	// Retrieve inode information for the file to be removed, including the parent and child inodes.
	parentInode, childInode, _, childInodeNum := getParentandChildInodes(fileName)
	if !childInode.IsValid {
		fmt.Println("File does not exist or inode is not valid.")
		return
	}

	// Check if the file has any associated data blocks.
	if childInode.DirectBlock1 == 0 {
		fmt.Println("Nothing to remove, the file is empty or unallocated.")
		return
	}

	// Attempt to unlink (remove) the file using the FileSystem package.
	err := FileSystem.Unlink(childInodeNum, parentInode)
	if err != nil {
		fmt.Printf("Error removing the file: %s\n", err)
		return
	}

	fmt.Println("File has been successfully removed!")
}

func appendToFile(fileName, content string) {
	// Retrieve the inode for the file to which the content will be appended.
	_, fileInode, _, fileInodeNum := getParentandChildInodes(fileName)
	if !fileInode.IsValid {
		fmt.Println("The specified file does not exist or the inode is not valid.")
		return
	}

	// Read existing content from the file.
	existingContent := FileSystem.Read(&fileInode)
	if existingContent == "" && fileInode.DirectBlock1 != 0 {
		fmt.Println("Failed to read existing content from the file.")
		return
	}

	// Combine existing content with new content.
	updatedContent := existingContent + content
	inputContent := []byte(updatedContent)

	// Write the combined content back to the file.
	FileSystem.Write(&fileInode, fileInodeNum, inputContent)

	fmt.Println("Content appended successfully.")
}

/*
Got the osCommand function from Dakotha Proffit, he helped me figure out how it works
*/

func osCommand(command string, args []string) {
	modifiedArgs := []string{}
	var nextOutputFile string
	pathInOutputFile := false

	// Iterate through arguments to handle redirection and check for paths
	for i, arg := range args {
		if arg == ">>" {
			if i+1 < len(args) {
				nextOutputFile = args[i+1] // Store the next argument after '>>'
				if strings.Contains(nextOutputFile, "/") {
					pathInOutputFile = true // Set flag if '/' is found
				}
			}
			break // Stop processing arguments after '>>'
		}
		modifiedArgs = append(modifiedArgs, arg) // Add argument if not part of redirection
	}

	inputFileContent, err := os.ReadFile(modifiedArgs[len(modifiedArgs)-1])
	if err != nil {
		fmt.Println("couldn't read in file")
		fmt.Println(err)
	}

	// Output relevant information or take actions based on flags here, if necessary
	if pathInOutputFile {
		stringSlice := strings.Split(nextOutputFile, "/")
		fileName := stringSlice[len(stringSlice)-1]
		parentinode, _, _, _ := getParentandChildInodes(nextOutputFile)
		newFileInode, firstInodeNun := FileSystem.Open(FileSystem.CREATE, fileName, parentinode)
		contentToWrite := []byte(inputFileContent)
		FileSystem.Write(&newFileInode, firstInodeNun, contentToWrite)
		fmt.Println("file read in")
	} else {
		newFileInode, firstInodeNun := FileSystem.Open(FileSystem.CREATE, nextOutputFile, FileSystem.RootFolder)
		contentToWrite := []byte(inputFileContent)
		FileSystem.Write(&newFileInode, firstInodeNun, contentToWrite)
		fmt.Println("file read in")
	}

}
