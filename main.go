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
	"net/http"
	"os"
	"os/user"
	"path"
	"runtime"
)

var Version string = "2.2"
var Options []string

var AppFolder = "/"
var AppDataFolder string = "/"
var AppDataTmpFolder string = "/tmp"
var AppFileName = ""

var LastBuildURL = "http://whitecatboard.org/lastbuildv2.php"
var FirmwareURL = "http://whitecatboard.org/firmwarev2.php"
var SupportedBoardsURL = "https://raw.githubusercontent.com/whitecatboard/Lua-RTOS-ESP32/master/boards/boards.json"

func usage() {
	fmt.Println("usage: wcc -p port | -ports [-ls path | [-down source destination] | [-up source destination] | [-f | -ffs] | [-erase] | -d]\r\n")
	fmt.Println("-ports:\t\t list all available serial ports on your computer")

	if runtime.GOOS == "windows" {
		fmt.Println("-p port:\t serial port, for example COM2")
	} else {
		fmt.Println("-p port:\t serial port device, for example /dev/tty.SLAB_USBtoUART")
	}

	fmt.Println("-ls path:\t list files present in path")
	fmt.Println("-down src dst:\t transfer the source file (board) to destination file (computer)")
	fmt.Println("-up src dst:\t transfer the source file (computer) to destination file (board)")
	fmt.Println("-f:\t\t flash board with last firmware")
	fmt.Println("-ffs:\t\t flash board with last filesystem")
	fmt.Println("-erase:\t\t erase flash board")
	fmt.Println("-d:\t\t show debug messages\r\n")
}

// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}

func main() {
	defer func() {
		if connectedBoard != nil {
			connectedBoard.detach()
		}

		if err := recover(); err != nil {
			fmt.Println("Error:", err)
		}
	}()

	port := ""
	ports := false
	dbg := false
	ok := true
	i := 0
	ls := false
	down := false
	up := false
	flash := false
	flashFS := false
	nextIsPort := false
	nextIsSrc := false
	nextIsDst := false
	nextIsDir := false
	erase := false
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

		case "-f":
			flash = true

		case "-ffs":
			flashFS = true

		case "-ports":
			ports = true

		case "-erase":
			erase = true

		default:
			if i > 0 {
				ok = false
			}
		}

		i = i + 1
	}

	if ports {
		fmt.Println("Available serial ports on your computer:\r\n")
		list_ports()
		os.Exit(1)
	}

	if (!erase && !up && !down && !ls && !(flash || flashFS)) || (port == "") {
		ok = false
	}

	if erase && (flash || flashFS) {
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

	// Clean tmp folder
	os.RemoveAll(AppDataTmpFolder + "/")

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
	if connectedBoard == nil {
		fmt.Println("Can't connect to any board at port " + port + ".\r\n")
		fmt.Println("Available serial ports on your computer:\r\n")
		list_ports()
		os.Exit(1)
	}

	if connectedBoard.validFirmware {
		connectedBoard.consoleOut = false
		connectedBoard.consoleIn = true
		connectedBoard.timeout(2000)
		connectedBoard.model = connectedBoard.sendCommand("do local type = os.board();print(type); end")
		connectedBoard.subtype = connectedBoard.sendCommand("do local _,subtype = os.board();if (not (subtype == nil)) then print(subtype); else print(\"\"); end end")
		connectedBoard.brand = connectedBoard.sendCommand("do local _,_,brand = os.board();if (not (brand == nil)) then print(brand); else print(\"\"); end end")
		connectedBoard.noTimeout()
		connectedBoard.consoleOut = true
		connectedBoard.consoleIn = false

		firmware := ""

		if connectedBoard.brand != "" {
			firmware = connectedBoard.brand + "-"
		}

		firmware = firmware + connectedBoard.model

		if connectedBoard.subtype != "" {
			firmware = firmware + "-" + connectedBoard.subtype
		}

		connectedBoard.firmware = firmware
	} else {
		connectedBoard.noTimeout()
	}

	if (connectedBoard.model == "") && !erase {
		conf := ""
		okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
		nokayResponses := []string{"n", "N", "no", "No", "NO"}

		fmt.Println("Unknown board model.")
		fmt.Println("Maybe your firmware is corrupted, or you haven't a valid Lua RTOS firmware installed.")

		for {
			fmt.Print("\nDo you want to install a valid firmware now [y/n])? ")

			_, err := fmt.Scanln(&conf)
			if err == nil {
				if containsString(okayResponses, conf) {
					for {
						fmt.Print("\r\n")
						connectedBoard.selectSupportedBoard()
						connectedBoard.upgrade(false, true, flashFS)
						notify("progress", "board upgraded\r\n")

						os.Exit(1)
					}
					os.Exit(1)
				} else if containsString(nokayResponses, conf) {
					os.Exit(1)
				}
			}

		}
	}

	if ls {
		connectedBoard.consoleOut = false
		connectedBoard.consoleIn = true
		connectedBoard.timeout(2000)
		response = connectedBoard.sendCommand("os.ls(\"" + dir + "\")")
		connectedBoard.noTimeout()
		connectedBoard.consoleOut = true
		connectedBoard.consoleIn = false
		fmt.Println(response)
	} else if down {
		file := connectedBoard.readFile(src)
		err := ioutil.WriteFile(dst, file, 0755)
		if err != nil {
			panic(err)
		}
	} else if up {
		file, err := ioutil.ReadFile(src)
		if err != nil {
			panic(err)
		}

		connectedBoard.writeFile(dst, file)
	} else if flash || flashFS {
		newBuild := false

		connectedBoard.consoleOut = false
		connectedBoard.consoleIn = true
		connectedBoard.timeout(2000)
		commit := connectedBoard.sendCommand("do local commit; _, _, _, commit = os.version();print(commit);end")
		connectedBoard.noTimeout()
		connectedBoard.consoleOut = true
		connectedBoard.consoleIn = false
		lastCommit := ""

		// Test for a new firmware version
		resp, err := http.Get(LastBuildURL + "?firmware=" + connectedBoard.firmware)
		if err == nil {
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				lastCommit = string(body)

				log.Println("current commit ", commit)
				log.Println("last commit ", lastCommit)

				if (commit != lastCommit) && (lastCommit != "") {
					newBuild = true
					notify("progress", "new firmware available "+commit+"\r\n")
				} else {
					notify("progress", "board is updated "+commit+"\r\n")
				}
			} else {
				panic(err)
			}
		} else {
			panic(err)
		}

		if newBuild || flashFS {
			connectedBoard.upgrade(false, newBuild && flash, flashFS)
			notify("progress", "board upgraded to "+lastCommit+"\r\n")
		}
	} else if erase {
		connectedBoard.upgrade(true, false, false)
		notify("progress", "Board erased           \r\n")
	}

	// Clean tmp folder
	os.RemoveAll(AppDataTmpFolder + "/")
}
