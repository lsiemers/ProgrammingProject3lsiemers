package FileSystem

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// Disk
// first index in number of blocks
// second is block size
// I need 6144 blocks for the data - one for the superblock
// I'll cheese the 'bitmaps' as booleans so I need 6144 bytes (6 blocks) for the data 'bitmap'
// and I'll need inodes and an inode bitmap. I'll setup my inodes to be 64 bytes and if
// I have 256 of them, then I need 64 blocks for inodes
// furthermore I'll need 1 block for the inode 'bitmap'

var Disk [66184][BLOCK_SIZE]byte
var RootFolder INode

const (
	INODE_SIZE       = 512 //even though Inodes are only 64 bytes, encoded they take up 170, and need power of 2
	BLOCK_SIZE       = 1024
	NUM_INODES       = 256
	DATA_BLOCK_START = 140
)

type SuperBlock struct {
	INodeStart       int //the block location of the beginning of the inodes
	RootDirInode     int //the inode number of the root folder
	FreeBlockStart   int //the block number where the beginning of the booleans for the free blocks is found
	InodeBitmapStart int //block number of the inode 'bitmap'
	DataBlockStart   int //the block number of the beginning of the datablocks
}

var RootInode *INode

type INode struct {
	IsValid        bool //true if this inode is a real file
	IsDirectory    bool //true if this file is actually a directory entry
	Version        int  //at the moment this is here mostly to make the inodes be 64 bytes
	DirectBlock1   int
	DirectBlock2   int
	DirectBlock3   int
	IndirectBlock  int
	CreateTime     int64
	LastModifyTime int64
	DirectBlocks   []int
	Children       map[string]*INode
}

type DirectoryEntry struct {
	Inode int
	Name  [20]byte //I suggested 12 in class, but I realize that 20 will make this an even 32 bytes
}

type DirectoryBlock [32]DirectoryEntry

type IndirectBlock [128]int

const (
	CREATE = iota
	READ
	WRITE
	APPEND
)

func InitializeFileSystem() {
	//explicitly zero the filesystem - this shouldn't be needed
	for blockLoc := range Disk {
		for byteLoc := range Disk[blockLoc] {
			Disk[blockLoc][byteLoc] = 0
		}
	}

	//order on the Disk will be Superblock in block 0, inode bitmap in block 1, free block bitmap  blocks 2-7
	//inodes in blocks 8-39 and datablocks in blocks 40-end

	supBlock := SuperBlock{
		INodeStart:       8,
		RootDirInode:     1,
		FreeBlockStart:   2,
		InodeBitmapStart: 1,
		DataBlockStart:   DATA_BLOCK_START,
	}
	superblockBytes := EncodeToBytes(supBlock)
	copy(Disk[0][:], superblockBytes)
	createInodeBitmap(supBlock)
	createFreeBlockBitmap(supBlock)
	createInodes(supBlock)
	createRootDir(supBlock)
}

func createFreeBlockBitmap(block SuperBlock) {
	//unlike the inode bitmap, the free block bitmap will take up multiple blocks
	wholeFreeBlockBitmap := make([][BLOCK_SIZE]bool, 1)
	for bitmapBlock := block.InodeBitmapStart; bitmapBlock < block.INodeStart; bitmapBlock++ {
		var currentFreeBlockBitmap [1024]bool //should be all false by default
		wholeFreeBlockBitmap = append(wholeFreeBlockBitmap, currentFreeBlockBitmap)
	}
	writeFreeBlockBitmapToDisk(wholeFreeBlockBitmap, block)
}

func createInodeBitmap(block SuperBlock) {
	//the inode bitmap will be in block 1 and will hold NUM_INODES booleans
	var inodeBitmap [NUM_INODES]bool //all set to zero by default
	writeInodeBitmapToDisk(inodeBitmap, block)
}

func createInodes(sblock SuperBlock) {
	//here we will create all 256/NUM_INODES INodes in the filesystem as invalid files
	//fix this
	for iNodeNum := 0; iNodeNum < NUM_INODES; iNodeNum++ {
		currentInode := INode{} //make empty with all fields having false/zero value
		inodeBytes := EncodeToBytes(currentInode)
		inodeblock := (iNodeNum * INODE_SIZE / BLOCK_SIZE) + sblock.INodeStart //this is all integer division, so result is floor division
		inodeOffSet := iNodeNum * INODE_SIZE % BLOCK_SIZE
		copy(Disk[inodeblock][inodeOffSet:inodeOffSet+INODE_SIZE], inodeBytes)
	}
}

func createRootDir(sblock SuperBlock) {
	//rather than reading the existing inode in, since I know they are all empty, I'll make a new one and write it to disk
	rootFolder := INode{
		IsValid:        true,
		IsDirectory:    true,
		Version:        0,
		DirectBlock1:   DATA_BLOCK_START + 1, //since this happens before any other allocation, just grab block 40
		DirectBlock2:   0,
		DirectBlock3:   0,
		IndirectBlock:  0,
		CreateTime:     time.Now().Unix(),
		LastModifyTime: time.Now().Unix(),
	}
	//now we need to mark the root inode as used
	inodeBitmap := ReadINodeBitmap(sblock)
	inodeBitmap[sblock.RootDirInode] = true //claim the inode for the root folder
	writeInodeBitmapToDisk(inodeBitmap, sblock)
	//and let's claim that direct block 40
	freeBlockBitmap := ReadFreeBlockBitmap(sblock)
	freeBlockBitmap[0][rootFolder.DirectBlock1] = true
	writeFreeBlockBitmapToDisk(freeBlockBitmap, sblock)
	rootBlock, _ := CreateDirectoryFile(0, sblock.RootDirInode)
	rootBlockBytes := EncodeToBytes(rootBlock)
	copy(Disk[rootFolder.DirectBlock1][:], rootBlockBytes)
	rootFolderAsBytes := EncodeToBytes(rootFolder)
	copy(Disk[sblock.INodeStart][INODE_SIZE*sblock.RootDirInode:INODE_SIZE*sblock.RootDirInode+INODE_SIZE], rootFolderAsBytes)
	RootFolder = rootFolder
}

func CreateDirectoryFile(parentInode int, folderinode int) (retBlock DirectoryBlock, currentInode INode) {
	if parentInode != 0 { //handle root directory specially, for all others, mark as folder now
		currentInode = getInodeFromDisk(folderinode) //we need to mark this as a folder now
		currentInode.IsDirectory = true
		if !currentInode.IsValid {
			currentInode.IsValid = true
		}
		writeInodeToDisk(&currentInode, folderinode, ReadSuperBlock())
	}
	dot := DirectoryEntry{
		Inode: folderinode,
	}
	dot.Name[0] = '.'
	dotdot := DirectoryEntry{
		Inode: parentInode,
	}
	dotdot.Name[0] = '.'
	dotdot.Name[1] = '.'
	return DirectoryBlock{dot, dotdot}, currentInode
}

func writeFreeBlockBitmapToDisk(bitmap [][BLOCK_SIZE]bool, sblock SuperBlock) {
	for loc, bitmapPart := range bitmap {
		for blockLoc, bit := range bitmapPart {
			if bit {
				Disk[loc+sblock.FreeBlockStart][blockLoc] = 1
			} else {
				Disk[loc+sblock.FreeBlockStart][blockLoc] = 0
			}
		}
	}
}

func ReadFreeBlockBitmap(sblock SuperBlock) [][BLOCK_SIZE]bool {
	//I decided to cheese this just a little to make life a little easier see below to do it right
	freeBlockBitmap := make([][BLOCK_SIZE]bool, sblock.INodeStart-sblock.FreeBlockStart)

	for bitmapBlockNum := sblock.FreeBlockStart; bitmapBlockNum < sblock.INodeStart; bitmapBlockNum++ {
		for bitLoc := 0; bitLoc < BLOCK_SIZE; bitLoc++ {
			if Disk[bitmapBlockNum][bitLoc] != 0 {
				freeBlockBitmap[bitmapBlockNum-sblock.FreeBlockStart][bitLoc] = true
			} else {
				freeBlockBitmap[bitmapBlockNum-sblock.FreeBlockStart][bitLoc] = false
			}
		}
	}
	return freeBlockBitmap
}

//this is my original - do it right version.
//func ReadFreeBlockBitmap(sblock SuperBlock) []bool {
//	freeBlockBitmap := make([]bool, BLOCK_SIZE)
//	for bitmapBlock := sblock.FreeBlockStart; bitmapBlock < sblock.INodeStart; bitmapBlock++ {
//		freeBlockBitmapPart := make([]bool, BLOCK_SIZE)
//		err := json.Unmarshal(Disk[bitmapBlock][:], &freeBlockBitmapPart)
//		if err != nil {
//			log.Fatal(err)
//		}
//		freeBlockBitmap = append(freeBlockBitmap, freeBlockBitmapPart...)
//	}
//	return freeBlockBitmap
//}

func writeInodeBitmapToDisk(bitmap [NUM_INODES]bool, sblock SuperBlock) {
	//I ended up having to copy bit by bit (bool by bool) there was no scope for being lazy
	for loc, bit := range bitmap {
		if bit {
			Disk[sblock.InodeBitmapStart][loc] = 1
		} else {
			Disk[sblock.InodeBitmapStart][loc] = 0
		}
	}
}

func ReadINodeBitmap(block SuperBlock) [NUM_INODES]bool {
	var iNodeBitmap [NUM_INODES]bool
	bitMapOnDisk := Disk[block.InodeBitmapStart]
	for bitNum := 0; bitNum < NUM_INODES; bitNum++ {
		iNodeBitmap[bitNum] = bitMapOnDisk[bitNum] != 0 //if the byte is zero, bit is false, non-zero is true
	}
	return iNodeBitmap
}

func ReadSuperBlock() SuperBlock {
	sBlock := SuperBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(Disk[0][:]))
	err := decoder.Decode(&sBlock)
	if err != nil {
		log.Fatal("Unable to Decode superblock - better blue Screen ", err)
	}
	return sBlock
}

// from https://gist.github.com/SteveBate/042960baa7a4795c3565
func EncodeToBytes(p interface{}) []byte {

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(p)
	if err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

// Open return values are first INodeStructure and second INode Number
func Open(mode int, name string, parentDir INode) (INode, int) {
	if !parentDir.IsDirectory || !parentDir.IsValid {
		log.Fatal("Tried to open file with invalid directory")
	}
	BlockWhereWeFindDirectoryEntry := parentDir.DirectBlock1 //I'm going to cheat here and only check direct block one since we would need more than 30 files otherwise
	DirectoryBlockBytes := Disk[BlockWhereWeFindDirectoryEntry]
	directoryEntryBlock := DirectoryBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(DirectoryBlockBytes[:]))
	err := decoder.Decode(&directoryEntryBlock)
	if err != nil {
		log.Fatal("Error decoding Directory block opening file ", name, ": ", err)
	}
	validDirectoryEntries := 0
	for _, entry := range directoryEntryBlock {
		//not really distinguishing read vs write here.
		if string(entry.Name[:len(name)]) == name {
			return getInodeFromDisk(entry.Inode), entry.Inode //if file is here, I'll just return it and the Inode Number for now
		}
		if entry.Inode == 0 && entry.Name[0] != '.' && entry.Name[1] != '.' { //once we get to invalid entries, get out of loop
			break
		}
		validDirectoryEntries++
	}
	//if we got here then the file wasn't in the directory
	if mode == CREATE {
		newInode, newInodeNum := createNewInode(ReadSuperBlock())
		newFile := DirectoryEntry{
			Inode: newInodeNum,
		}
		for num, char := range name {
			if num >= 20 {
				break
			}
			newFile.Name[num] = byte(char)
		}
		directoryEntryBlock[validDirectoryEntries] = newFile
		//write the directory entry back to the disk block
		currentDirectoryBlockBytes := EncodeToBytes(directoryEntryBlock)
		copy(Disk[parentDir.DirectBlock1][:], currentDirectoryBlockBytes)
		return newInode, newInodeNum
	}
	return INode{}, 0 //if we got here, return invalid/0 inode
}

// return value will be the INode data structure, and the Inode Number
func createNewInode(sBlock SuperBlock) (INode, int) {
	inodeBitmap := ReadINodeBitmap(sBlock)
	freeInodeLoc := sBlock.RootDirInode               //we will begin looking for a free inode starting with the root node
	for ; freeInodeLoc < NUM_INODES; freeInodeLoc++ { //there are only 25 possible inodes
		if inodeBitmap[freeInodeLoc] == false { //once we find an unused one stop
			inodeBitmap[freeInodeLoc] = true
			break
		}
	}
	if freeInodeLoc >= 511 {
		log.Fatal("All out of Inodes") //in a real file system I would return the 0/invalid inode
	}
	writeInodeBitmapToDisk(inodeBitmap, sBlock) 
	newInode := INode{
		IsValid:        true,
		IsDirectory:    false,
		Version:        0,
		DirectBlock1:   0,
		DirectBlock2:   0,
		DirectBlock3:   0,
		IndirectBlock:  0,
		CreateTime:     time.Now().Unix(),
		LastModifyTime: time.Now().Unix(),
	}
	writeInodeToDisk(&newInode, freeInodeLoc, sBlock)
	return newInode, freeInodeLoc
}

func writeInodeToDisk(inode *INode, InodeNum int, sblock SuperBlock) {
	InodeAsBytes := EncodeToBytes(inode)
	InodeBlock := InodeNum / (BLOCK_SIZE / INODE_SIZE) 
	InodeLocInBlock := InodeNum % (BLOCK_SIZE / INODE_SIZE)
	copy(Disk[sblock.INodeStart+InodeBlock][INODE_SIZE*InodeLocInBlock:INODE_SIZE*InodeLocInBlock+INODE_SIZE], InodeAsBytes)
}

func getInodeFromDisk(inodeNum int) INode {
	INodeBlock := inodeNum / (BLOCK_SIZE / INODE_SIZE)
	InodeOffset := inodeNum % (BLOCK_SIZE / INODE_SIZE)
	sblock := ReadSuperBlock()
	InodeFromDisk := INode{}
	InodeAsBytes := Disk[sblock.INodeStart+INodeBlock][InodeOffset*INODE_SIZE : (InodeOffset*INODE_SIZE)+INODE_SIZE]
	decoder := gob.NewDecoder(bytes.NewReader(InodeAsBytes))
	err := decoder.Decode(&InodeFromDisk)
	if err != nil {
		log.Fatal("Error decoding Inode ", inodeNum, " from disk - better blue Screen", err)
	}
	return InodeFromDisk
}

func Unlink(inodeNumToDelete int, parentDir INode) error {
	blockWhereDirectoryEntryIsFound := parentDir.DirectBlock1
	directoryBlockBytes := Disk[blockWhereDirectoryEntryIsFound]
	directoryEntryBlock := DirectoryBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(directoryBlockBytes[:]))

	if err := decoder.Decode(&directoryEntryBlock); err != nil {
		return fmt.Errorf("error decoding directory block: %w", err)
	}

	// Attempt to find and clear the inode entry
	found := false
	for i, entry := range directoryEntryBlock {
		if entry.Inode == inodeNumToDelete {
			directoryEntryBlock[i] = DirectoryEntry{}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("inode %d not found in directory", inodeNumToDelete)
	}

	// Update the inode bitmap and inode structure
	inodeBitmap := ReadINodeBitmap(ReadSuperBlock())
	inodeBitmap[inodeNumToDelete] = false
	writeInodeBitmapToDisk(inodeBitmap, ReadSuperBlock())

	inodeStruct := getInodeFromDisk(inodeNumToDelete)
	inodeStruct.IsValid = false
	writeInodeToDisk(&inodeStruct, inodeNumToDelete, ReadSuperBlock())

	// Write the updated directory block back to disk
	updatedDirectoryBlockBytes := EncodeToBytes(directoryEntryBlock)
	copy(Disk[blockWhereDirectoryEntryIsFound][:], updatedDirectoryBlockBytes)

	return nil
}

func Read(file *INode) string {
	if !file.IsValid || file.IsDirectory {
		fmt.Println("File is invalid or a directory")
		return ""
	}
	fileContents := strings.Builder{}
	fmt.Printf("Reading direct block 1: %d\n", file.DirectBlock1)
	firstBlock := Disk[file.DirectBlock1]
	fileContents.Write(firstBlock[:])
	fmt.Printf("Content from block 1: '%s'\n", string(firstBlock[:]))

	if file.DirectBlock2 == 0 {
		return fileContents.String()
	}
	fmt.Printf("Reading direct block 2: %d\n", file.DirectBlock2)
	secondBlock := Disk[file.DirectBlock2]
	fileContents.Write(secondBlock[:])
	fmt.Printf("Content from block 2: '%s'\n", string(secondBlock[:]))

	if file.DirectBlock3 == 0 {
		return fileContents.String()
	}
	fmt.Printf("Reading direct block 3: %d\n", file.DirectBlock3)
	thirdBlock := Disk[file.DirectBlock3]
	fileContents.Write(thirdBlock[:])
	fmt.Printf("Content from block 3: '%s'\n", string(thirdBlock[:]))

	if file.IndirectBlock == 0 {
		return fileContents.String()
	}
	fmt.Printf("Reading from indirect block: %d\n", file.IndirectBlock)
	indirectBlockVal := getIndirectBlock(file)
	for i, blockNum := range indirectBlockVal {
		if blockNum == 0 {
			break
		}
		fmt.Printf("Reading indirect block number %d: %d\n", i+1, blockNum)
		blockData := Disk[blockNum][:]
		fileContents.Write(blockData)
		fmt.Printf("Content from indirect block %d: '%s'\n", i+1, string(blockData))
	}
	return fileContents.String()
}

func Write(file *INode, inodeNum int, content []byte) {
	file.LastModifyTime = time.Now().Unix() //update last modify time
	numCompleteBlocks := len(content) / BLOCK_SIZE
	hasLeftovers := len(content)%BLOCK_SIZE > 0
	block := 0
	for ; block < numCompleteBlocks; block++ {
		if block == 0 {
			if file.DirectBlock1 == 0 {
				file.DirectBlock1 = allocateNewBlock(ReadSuperBlock())
			}
			blockEnd := BLOCK_SIZE * (block + 1)
			if blockEnd > len(content) {
				blockEnd = len(content)
			}
			copy(Disk[file.DirectBlock1][:], content[BLOCK_SIZE*block:blockEnd])
		} else if block == 1 {
			if file.DirectBlock2 == 0 {
				file.DirectBlock2 = allocateNewBlock(ReadSuperBlock())
			}
			blockEnd := BLOCK_SIZE * (block + 1)
			if blockEnd > len(content) {
				blockEnd = len(content)
			}
			copy(Disk[file.DirectBlock2][:], content[BLOCK_SIZE*block:blockEnd])
		} else if block == 2 {
			if file.DirectBlock3 == 0 {
				file.DirectBlock3 = allocateNewBlock(ReadSuperBlock())
			}
			blockEnd := BLOCK_SIZE * (block + 1)
			if blockEnd > len(content) {
				blockEnd = len(content)
			}
			copy(Disk[file.DirectBlock3][:], content[BLOCK_SIZE*block:blockEnd])
		} else {
			indirectBlockVal := getIndirectBlock(file)
			for indirectBlockNum, blockLoc := range indirectBlockVal {
				blockEnd := BLOCK_SIZE * (block + 1)
				if blockEnd > len(content) {
					blockEnd = len(content)
				}
				if blockLoc != 0 {
					copy(Disk[blockLoc][:], content[BLOCK_SIZE*block:BLOCK_SIZE*(block+1)])
					block++
					if block >= numCompleteBlocks {
						break
					}
				} else {
					newBlock := allocateNewBlock(ReadSuperBlock())
					indirectBlockVal[indirectBlockNum] = newBlock
					//write the actual data to disk
					copy(Disk[newBlock][:], content[BLOCK_SIZE*block:blockEnd])
					block++
				}
			}
			indirectBlockBytes := EncodeToBytes(indirectBlockVal)
			copy(Disk[file.IndirectBlock][:], indirectBlockBytes)
		}
	}
	if hasLeftovers {
		leftovers := content[(len(content)/BLOCK_SIZE)*block:]
		if numCompleteBlocks == 0 {
			if file.DirectBlock1 == 0 {
				file.DirectBlock1 = allocateNewBlock(ReadSuperBlock())
			}
			copy(Disk[file.DirectBlock1][:], leftovers)
		} else if numCompleteBlocks == 1 {
			copy(Disk[file.DirectBlock2][:], leftovers)
		} else if numCompleteBlocks == 2 {
			copy(Disk[file.DirectBlock3][:], leftovers)
		} else {
			indirectBlockVal := getIndirectBlock(file)
			finalBlockLoc := indirectBlockVal[numCompleteBlocks-3] 
			if finalBlockLoc != 0 {
				copy(Disk[finalBlockLoc][:], leftovers)
			} else {
				newBlock := allocateNewBlock(ReadSuperBlock())
				indirectBlockVal[numCompleteBlocks-3] = newBlock
				indirectBlockBytes := EncodeToBytes(indirectBlockVal)
				//write the indirect block to disk
				copy(Disk[file.IndirectBlock][:], indirectBlockBytes)
				//write the actual data to disk
				copy(Disk[newBlock][:], leftovers)
			}
		}
	}
	writeInodeToDisk(file, inodeNum, ReadSuperBlock())
}

// returns location of newly allocated block
func allocateNewBlock(sblock SuperBlock) int {
	freeBlockBitmap := ReadFreeBlockBitmap(sblock)
	blockNum := RootFolder.DirectBlock1 
	for bitblock, bitmapBlock := range freeBlockBitmap {
		for locInBlock, bit := range bitmapBlock {
			if bitblock == 0 && locInBlock <= blockNum {
				continue 
			}
			if !bit {
				//this bit is available
				freeBlockBitmap[bitblock][locInBlock] = true
				writeFreeBlockBitmapToDisk(freeBlockBitmap, sblock)
				return locInBlock
			} else {

			}
		}
	}
	log.Fatal("Unable to allocate a free block")
	return 0
}

func getIndirectBlock(file *INode) IndirectBlock {
	if file.IndirectBlock == 0 {
		file.IndirectBlock = allocateNewBlock(ReadSuperBlock())
		return IndirectBlock{}
	}
	//now we need to do the indirect blocks
	indirectBlockBytes := getIndirectBlockFromDisk(file.IndirectBlock)
	indirectBlockVal := IndirectBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(indirectBlockBytes[:]))
	err := decoder.Decode(&indirectBlockVal)
	if err != nil { 
		log.Fatal("Error decoding IndirectBlock from disk - better blue Screen", err)
	}
	return indirectBlockVal
}

func getIndirectBlockFromDisk(indirectBlockNum int) [1024]byte {
	return Disk[indirectBlockNum]
}

func DecodeDirectoryBlock(blockNum int) (DirectoryBlock, error) {
	if blockNum < 0 || blockNum >= len(Disk) {
		return DirectoryBlock{}, fmt.Errorf("block number %d out of range", blockNum)
	}

	blockData := Disk[blockNum][:]
	var dirBlock DirectoryBlock
	decoder := gob.NewDecoder(bytes.NewReader(blockData))
	if err := decoder.Decode(&dirBlock); err != nil {
		return DirectoryBlock{}, fmt.Errorf("error decoding directory block: %w", err)
	}

	return dirBlock, nil
}
func GetINodeDetails(path string) (inode INode, inodeNum int, err error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return INode{}, 0, fmt.Errorf("invalid path")
	}

	
	inode = INode{IsValid: true, IsDirectory: false, DirectBlock1: 1} // Simulated inode data
	inodeNum = 1                                                      // Simulated inode number

	return inode, inodeNum, nil
}

func FindSubdirectories(dir string) (INode, int) {
	stringSlice := strings.Split(dir, "/")
	parentNode := RootFolder
	parentNodeNum := 0
	for _, str := range stringSlice {
		parentNode, parentNodeNum = Open(READ, str, parentNode)
		if parentNodeNum == 0 {
			log.Fatal("Location not found")
			return INode{}, 0
		}
	}
	return parentNode, parentNodeNum
}
func GetInodeFromPath(path string) (*INode, error) {
	fmt.Println("Path:", path) // Print the path
	if path == "" {
		return RootInode, nil
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return nil, errors.New("invalid path")
	}

	// Start at the root directory
	currentInode := RootInode

	// Traverse the path
	for _, part := range parts {
		fmt.Println("Part:", part) // Print the part
		if !currentInode.IsDirectory {
			return nil, errors.New("not a directory: " + part)
		}

		// Look up the next part in the current directory
		nextInode, ok := currentInode.Children[part]
		if !ok {
			return nil, errors.New("no such file or directory: " + part)
		}

		currentInode = nextInode
	}

	return currentInode, nil
}
