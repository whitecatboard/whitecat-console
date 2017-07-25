/*
 * Whitecat Console
 *
 * Copyright (C) 2015 - 2016
 * IBEROXARXA SERVICIOS INTEGRALES, S.L.
 *
 * Author: Jaume Olivé (jolive@iberoxarxa.com / jolive@whitecatboard.org)
 *
 * All rights reserved.
 *
 * Permission to use, copy, modify, and distribute this software
 * and its documentation for any purpose and without fee is hereby
 * granted, provided that the above copyright notice appear in all
 * copies and that both that the copyright notice and this
 * permission notice and warranty disclaimer appear in supporting
 * documentation, and that the name of the author not be used in
 * advertising or publicity pertaining to distribution of the
 * software without specific, written prior permission.
 *
 * The author disclaim all warranties with regard to this
 * software, including all implied warranties of merchantability
 * and fitness.  In no event shall the author be liable for any
 * special, indirect or consequential damages or any damages
 * whatsoever resulting from loss of use, data or profits, whether
 * in an action of contract, negligence or other tortious action,
 * arising out of or in connection with the use or performance of
 * this software.
 */

package main

import (
	"fmt"
	"github.com/kardianos/osext"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"runtime"
)

var Version string = "1.1"
var Options []string

var AppFolder = "/"
var AppDataFolder string = "/"
var AppDataTmpFolder string = "/tmp"
var AppFileName = ""

func usage() {
	fmt.Println("wcc: usage: wccagent -p port [-ls path | -down source destination | -up source destination | -d]")
	fmt.Println("")
	fmt.Println("-p port:\t serial port device, for example /dev/tty.SLAB_USBtoUART")
	fmt.Println("-ls path:\t list files present in path")
	fmt.Println("-down src dst:\t transfer the source file (board) to destination file (computer)")
	fmt.Println("-up src dst:\t transfer the source file (computer) to destination file (board)")
	fmt.Println("-d:\t\t show debug messages")
	fmt.Println("")
}

func main() {
	defer func() {
		if connectedBoard != nil {
			connectedBoard.detach()
		}
	}()

	port := ""
	dbg := false
	ok := true
	i := 0
	ls := false
	down := false
	up := false
	nextIsPort := false
	nextIsSrc := false
	nextIsDst := false
	nextIsDir := false
	src := ""
	dst := ""
	dir := ""
	response := ""

	// Get arguments and process arguments
	for _, arg := range os.Args {
		if nextIsDir {
			dir = arg
			nextIsDir = false
			continue
		}

		if nextIsSrc {
			src = arg
			nextIsSrc = false
			nextIsDst = true
			continue
		}

		if nextIsDst {
			dst = arg
			nextIsDst = false
			continue
		}

		if nextIsPort {
			port = arg
			nextIsPort = false
			continue
		}
		
		switch arg {
		case "-p":
			port = arg
			nextIsPort = true
			
		case "-ls":
			ls = true
			nextIsDir = true

		case "-down":
			down = true
			nextIsSrc = true

		case "-up":
			up = true
			nextIsSrc = true
			
		case "-d":
			dbg = true
		
		default:
			if i > 0 {
				ok = false
			}
		}

		i = i + 1
	}
	
	if !up && !down && !ls {
		ok = false
	}

	if !ok {
		usage()
		os.Exit(1)
	}

	if !dbg {
		log.SetOutput(ioutil.Discard)
	}

	// Get home directory, create the user data folder, and needed folders
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	if runtime.GOOS == "darwin" {
		AppDataFolder = path.Join(usr.HomeDir, ".wccagent")
	} else if runtime.GOOS == "windows" {
		AppDataFolder = path.Join(usr.HomeDir, "AppData", "The Whitecat Create Agent")
	} else if runtime.GOOS == "linux" {
		AppDataFolder = path.Join(usr.HomeDir, ".whitecat-create-agent")
	}

	AppDataTmpFolder = path.Join(AppDataFolder, "tmp")

	_ = os.Mkdir(AppDataFolder, 0755)
	_ = os.Mkdir(AppDataTmpFolder, 0755)

	// Get where program is executed
	execFolder, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}

	AppFolder = execFolder
	AppFileName, _ = osext.Executable()

	log.Println("AppFolder: ", AppFolder)
	log.Println("AppFileName: ", AppFileName)
	log.Println("AppDataFolder: ", AppDataFolder)
	log.Println("AppDataTmpFolder: ", AppDataTmpFolder)

	if ls {
		if dir == "" {
			usage()
			os.Exit(1)
		}
	} else if down {
		if src == "" || dst == "" {
			usage()
			os.Exit(1)
		}
	} else if up {
		if src == "" || dst == "" {
			usage()
			os.Exit(1)
		}
	}
	
	// Connect board
	connect(port)
	
	if ls {
		connectedBoard.consoleOut = false
		connectedBoard.consoleIn = true
		connectedBoard.timeout(2000)
		response = connectedBoard.sendCommand("os.ls(\""+ dir + "\")")
		connectedBoard.noTimeout()
		connectedBoard.consoleOut = true
		connectedBoard.consoleIn = false		
		fmt.Println(response)
	} else if down {
		file := connectedBoard.readFile(src)
		err := ioutil.WriteFile(src, file, 0755)
		if err != nil {
			panic(err)
		}
	} else if up {
		file, err := ioutil.ReadFile(src)
		if err != nil {
			panic(err)
		}

		connectedBoard.writeFile(dst, file)
	}
}