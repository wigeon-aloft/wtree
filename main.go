package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

const (
	// File types
	Regular FileType = iota
	Directory
	Link
	Socket
	Pipe
)

type FileType int

func (f FileType) String() string {
	return fileTypeLookup[f]
}

var (
	positionalArgs []string

	allTypes = []FileType{Regular, Directory, Link, Socket, Pipe}
	maxDepth int = math.MaxInt // -1 is placeholder for 'inifinite' depth
	types []FileType // The user's chosen types
	noHidden bool
	showPerms bool
	showOwner bool
	showGroup bool
	showType bool

	fileTypeLookup = map[FileType]string{
		Regular: "regular",
		Directory: "directory",
		Link: "link",
		Socket: "socket",
		Pipe: "pipe",
	}
)

var fileSymbolToTypeLookup = map[string]FileType{
	"f": Regular,
	"d": Directory,
	"l": Link,
	"s": Socket,
	"p": Pipe,
}

var fileTypeToSymbolLookup = map[FileType]string{}

func usage() {
	fmt.Println(os.Args[0], "[\x1B[4mPATH\x1B[0m]... [\x1B[4mOPTION\x1B[0m]...")
	fmt.Println()
	fmt.Println("\x1b[1mDESCRIPTION\x1b[22m")
	fmt.Println()
	fmt.Println("\tResursively prints a tree diagram of the directory structure under the")
	fmt.Println("\tspecified \x1B[4mPATH\x1B[0m. If no \x1B[4mPATH\x1B[0m is provided the current directory is used.")
	fmt.Println()
	fmt.Println("\tAll POSIX-style '--' options can also be used with a single hyphen '-'.")
	fmt.Println()
	fmt.Println("\x1b[1mOPTIONS\x1b[22m")
	fmt.Println()
	fmt.Println("\t-h, --help\tShow this message")
	fmt.Println()
	fmt.Println("\t-d, --depth [\x1B[4mDEPTH\x1B[0m]\t\n\t\t\tShow files up to \x1B[4mDEPTH\x1B[0m directory levels deep")
	fmt.Println()
	fmt.Println("\t-t, --type [\x1B[4mTYPE\x1B[0m]...\n\t\t\tSelect a filetype to filter for. Can be specified")
	fmt.Println("\t\t\tmultiple times. Valid options are:")
	fmt.Println("\t\t\t\t* f: regular file")
	fmt.Println("\t\t\t\t* d: directory")
	fmt.Println("\t\t\t\t* l: link")
	fmt.Println("\t\t\t\t* s: socket")
	fmt.Println("\t\t\t\t* p: named pipe")
	fmt.Println("\t\t\tIf this option isn't specified, the default is to show")
	fmt.Println("\t\t\tall file types. Note that parent directories are always")
	fmt.Println("\t\t\tprinted if they contain a file type that is filtered for.")
	fmt.Println()
	fmt.Println("\t-p, --perm\tShow file permissions")
	fmt.Println()
	fmt.Println("\t-o, --owner\tShow file owner")
	fmt.Println()
	fmt.Println("\t-g, --group\tShow file group")
	fmt.Println()
	fmt.Println("\t-y, --show-type\tShow type symbol of file in output")
	fmt.Println()
	fmt.Println("\t-a, --no-hidden\tHide hidden files")
	fmt.Println()
}

func GetFileTypeFromSymbol(symbol string) FileType {
	return fileSymbolToTypeLookup[symbol]
}

// I know you can get this info an easier way, I just thought this would be a fun
// exercise in bit manipulation.
func getFileType(fileMode os.FileMode) FileType {
	var mode uint32
	// 8th bit set indicates file is a socket
	mode = uint32(fileMode) >> 24
	if mode & 1 == 1 {
		return Socket
	}
	// 7th bit set indicates file is a pipe
	mode = uint32(fileMode) >> 25
	if mode & 1 == 1 {
		return Pipe
	}
	// 5th bit set indicates file is a link
	mode = uint32(fileMode) >> 27
	if mode & 1 == 1 {
		return Link
	}
	// 1st bit set indicates a directory
	mode = uint32(fileMode) >> 31
	if mode & 1 == 1 {
		return Directory
	}

	return Regular
}

func getFilePermsOctalString(fileMode os.FileMode) string {
	permsMasked := fileMode & 0b1_11111111
	return fmt.Sprintf("%o", permsMasked)
}

func lookUpOwnerGroup(filePath string) (string, string) {
	fstat := syscall.Stat_t{}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
	}

	err = syscall.Fstat(int(file.Fd()), &fstat)
	if err != nil {
		fmt.Println(err)
	}

	return string(fstat.Uid), string(fstat.Gid)
}


// Recursively walks through a directory structure, up to maximum depth
// FIX: Return the number of files printed? Then that number can be used to determine if a directory's
// name should be printed based on if it contained any files types that were filtered-for.
func walk(location string, depth int, drawDepths []int) int {
	var isLast bool = false
	var numFilesPrinted int

	if depth == maxDepth {
		return numFilesPrinted
	}

	if _, err := os.Stat(location); errors.Is(err, os.ErrNotExist) {
		fmt.Println(location, "does not exist")
		return numFilesPrinted
	}

	dirContents, err := os.ReadDir(location)
	if err != nil {
		log.Println("unable to read content of", location, ":", err)
	}

	for idx, dirEntry := range dirContents {
		var outputFormat string
		filePath := path.Join(location, dirEntry.Name())

		fileInfo, _ := dirEntry.Info()
		fileType := getFileType(fileInfo.Mode())

		// Don't bother continuing processing if the filetype of the current file hasn't been filtered for
		// by the user. Special case for directories, as we may want to print them if they contain types 
		// that have been filtered for.
		if !slices.Contains(types, fileType) && fileType != Directory {
			continue
		}

		if dirEntry.Name()[0] == '.' && noHidden {
			continue
		}

		for idx := range depth {
			if slices.Contains(drawDepths, idx) {
				outputFormat += "│  "
			} else {
				outputFormat += "   "
			}
		}

		if idx == len(dirContents)-1 {
			// FIX: This breaks if any file types are filtered-for, as the last file printed
			// will not be the same as the last file in the dirContents slice
			outputFormat += "└─"
			indexOfDepth := slices.Index(drawDepths, depth)
			if indexOfDepth != -1 {
				drawDepths = append(drawDepths[:indexOfDepth], drawDepths[indexOfDepth+1:]...)
			}
			isLast = true
		} else {
			outputFormat += "├─"
		}

		formatStart := ""
		formatEnd := ""
		switch fileType {
		case Directory:
			// make it bold
			formatStart = "\x1B[1m"
			formatEnd = "\x1B[22m"
		case Link:
			// make it green
			formatStart = "\x1B[32m"
			formatEnd = "\x1B[0m"
		case Socket:
			// make it cyan
			formatStart = "\x1B[36m"
			formatEnd = "\x1B[0m"
		case Pipe:
			// make it magenta
			formatStart = "\x1B[35m"
			formatEnd = "\x1B[0m"
		}

		if showPerms {
			outputFormat += fmt.Sprintf(" [%s]", getFilePermsOctalString(fileInfo.Mode()))
		}

		// if showOwner || showGroup {
		// 	owner, group := lookUpOwnerGroup(filePath)
		// 	if !showOwner {
		// 		owner = ""
		// 	} else {
		// 		group = ""
		// 	}
		// 	outputFormat += fmt.Sprintf(" %s:%s", owner, group)
		// }

		if showType {
			outputFormat += fmt.Sprintf(
				" \x1B[2m%s\x1B[0m",
				fileTypeToSymbolLookup[fileType],
			)
		}

		// Add the link target to the output
		outputFormat += " %s%s%s"

		if fileType == Link {
			linkTarget, err := os.Readlink(filePath)
			if err != nil {
				fmt.Println(err)
			}
			outputFormat += fmt.Sprintf(" ► %s", linkTarget)
		}

		fmt.Printf(
			outputFormat + "\n",
			formatStart,
			fileInfo.Name(),
			formatEnd,
		)
		if fileType != Directory {
			numFilesPrinted++
		}

		// Call walk recursively to descend through directories
		if fileType == Directory {
			if !isLast && !slices.Contains(drawDepths, depth) {
				drawDepths = append(drawDepths, depth)
			}

			recurFilesPrinted := walk(
				filePath,
				depth + 1,
				drawDepths,
			)
			// fmt.Println(recurFilesPrinted)

			// If the user has not filtered for directories and the directory
			// we just printed contained no files that were filtered for, 
			// delete the directory's line from the screen.
			if recurFilesPrinted == 0 && !slices.Contains(types, Directory) {
				fmt.Println("\x1B[A\x1B[2K\x1B[A")
			}

			numFilesPrinted += recurFilesPrinted
		}
	}

	return numFilesPrinted
}

func main() {
	for k, v := range fileSymbolToTypeLookup {
		fileTypeToSymbolLookup[v] = k
	}

	var optionValue string

	for idx := 1; idx <= len(os.Args[1:]); idx++ {
		arg := os.Args[idx]

		// Get positional arguments
		if arg[0] != '-' {
			positionalArgs = append(positionalArgs, arg)
			continue
		}

		// Process options (args starting with '-')
		option := strings.TrimLeft(arg, "-")
		if idx+1 < len(os.Args) {
			optionValue = os.Args[idx+1] 
		} else {
			optionValue = ""
		}
		switch option {
			case "help", "h":
				usage()
				os.Exit(0)
			case "depth", "d":
				var err error
				maxDepth, err = strconv.Atoi(os.Args[idx+1])
				if err != nil {
					log.Fatal("unable to parse depth value: ", err)
				}
			case "type", "t":
				fileType, ok := fileSymbolToTypeLookup[optionValue]
				if !ok {
					log.Fatalf("'%s' is not a valid type", optionValue)
				}
				types = append(types, fileType)
			case "perm", "p":
				showPerms = true
			case "owner", "o":
				showOwner = true
			case "group", "g":
				showGroup = true
			case "show-type", "y":
				showType = true
			case "no-hidden", "a":
				noHidden = true
			default:
				usage()
				fmt.Println("unknown option", arg)
				os.Exit(1)
		}

		// increment the count to skip the argument value
		if optionValue != "" {
			idx++
		}
	}

	// Remove duplicates from types
	if len(types) > 0 {
		slices.Sort(types)
		types = slices.Compact(types)
	} else {
		// User didn't specify any types, so default to all of them
		types = allTypes
	}

	if len(positionalArgs) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		positionalArgs = append(positionalArgs, cwd)
	}
	for _, positionalArg := range positionalArgs {

		if _, err := os.Stat(positionalArg); errors.Is(err, os.ErrNotExist) {
			fmt.Println(err)
		} else {
			fmt.Println("\x1b[1m" + positionalArg + "\x1b[22m")
			drawDepths := []int{1}
			walk(positionalArg, 0, drawDepths)
		}

	}
}
