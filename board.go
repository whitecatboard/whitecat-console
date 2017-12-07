/*
 * Whitecat Blocky Environment, board abstraction
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
 * and fitness.  In no events shall the author be liable for any
 * special, indirect or consequential damages or any damages
 * whatsoever resulting from loss of use, data or profits, whether
 * in an action of contract, negligence or other tortious action,
 * arising out of or in connection with the use or performance of
 * this software.
 */

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mikepb/go-serial"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SupportedBoard struct {
	Id           string
	Description  string
	Manufacturer string
	Brand        string
	Type         string
	Subtype      string
}

type SupportedBoards []SupportedBoard

var Upgrading bool

type Board struct {
	// Serial port
	port *serial.Port
	devInfo *serial.Info
	
	// Device name
	dev string

	// Is there a new firmware build?
	newBuild bool

	// Board information
	info string

	// Board model
	model    string
	subtype  string
	brand    string
	ota      bool
	firmware string

	// RXQueue
	RXQueue chan byte

	// Chunk size for send / receive files to / from board
	chunkSize int

	// If true disables notify board's boot events
	disableInspectorBootNotify bool

	consoleOut bool
	consoleIn  bool

	quit chan bool

	// Current timeout value, in milliseconds for read
	timeoutVal int

	// Firmware is valid?
	validFirmware bool
}

type BoardInfo struct {
	Build   string
	Commit  string
	Board   string
	Subtype string
	Brand   string
	Ota     bool
}

func (board *Board) timeout(ms int) {
	board.timeoutVal = ms
}

func (board *Board) noTimeout() {
	board.timeoutVal = math.MaxInt32
}

// Inspects the serial data received for a board in order to find special
// special events, such as reset, core dumps, exceptions, etc ...
//
// Once inspected all bytes are send to RXQueue channel
func (board *Board) inspector() {
	var re *regexp.Regexp

	defer func() {
		log.Println("stop inspector ...")

		if err := recover(); err != nil {
		}
	}()

	log.Println("start inspector ...")

	buffer := make([]byte, 1)

	line := ""

	for {
		if n, err := board.port.Read(buffer); err != nil {
			panic(err)
		} else {
			if n > 0 {
				if buffer[0] == '\n' {
					if !board.disableInspectorBootNotify {
						re = regexp.MustCompile(`^rst:.*\(POWERON_RESET\),boot:.*(.*)$`)
						if re.MatchString(line) {
							notify("boardPowerOnReset", "")
						}

						re = regexp.MustCompile(`^rst:.*(SW_CPU_RESET),boot:.*(.*)$`)
						if re.MatchString(line) {
							notify("boardSoftwareReset", "")
						}

						re = regexp.MustCompile(`^rst:.*(DEEPSLEEP_RESET),boot.*(.*)$`)
						if re.MatchString(line) {
							notify("boardDeepSleepReset", "")
						}

						re = regexp.MustCompile(`\<blockStart,(.*)\>`)
						if re.MatchString(line) {
							parts := re.FindStringSubmatch(line)
							info := "\"block\": \"" + base64.StdEncoding.EncodeToString([]byte(parts[1])) + "\""
							notify("blockStart", info)
						}

						re = regexp.MustCompile(`\<blockEnd,(.*)\>`)
						if re.MatchString(line) {
							parts := re.FindStringSubmatch(line)
							info := "\"block\": \"" + base64.StdEncoding.EncodeToString([]byte(parts[1])) + "\""
							notify("blockEnd", info)
						}

						re = regexp.MustCompile(`\<blockError,(.*),(.*)\>`)
						if re.MatchString(line) {
							parts := re.FindStringSubmatch(line)
							info := "\"block\": \"" + base64.StdEncoding.EncodeToString([]byte(parts[1])) + "\", " +
								"\"error\": \"" + base64.StdEncoding.EncodeToString([]byte(parts[2])) + "\""

							notify("blockError", info)
						}
					}

					// Remove prompt from line
					tmpLine := line
					re = regexp.MustCompile(`^/.*>\s`)
					tmpLine = string(re.ReplaceAll([]byte(tmpLine), []byte("")))

					re = regexp.MustCompile(`^([\/\.\/\-_a-zA-Z]*):(\d*)\:\s(\d*)\:(.*)$`)
					if re.MatchString(tmpLine) {
						parts := re.FindStringSubmatch(tmpLine)

						info := "\"where\": \"" + parts[1] + "\", " +
							"\"line\": \"" + parts[2] + "\", " +
							"\"exception\": \"" + parts[3] + "\", " +
							"\"message\": \"" + base64.StdEncoding.EncodeToString([]byte(parts[4])) + "\""
						log.Println(parts[4])

						re = regexp.MustCompile(`^WARNING\s.*$`)
						if re.MatchString(parts[4]) {
							notify("boardRuntimeWarning", info)
						} else {
							notify("boardRuntimeError", info)
						}
					} else {
						re = regexp.MustCompile(`^([\/\.\/\-_a-zA-Z]*)\:(\d*)\:\s*(.*)$`)
						if re.MatchString(tmpLine) {
							parts := re.FindStringSubmatch(tmpLine)

							info := "\"where\": \"" + parts[1] + "\", " +
								"\"line\": \"" + parts[2] + "\", " +
								"\"exception\": \"0\", " +
								"\"message\": \"" + base64.StdEncoding.EncodeToString([]byte(parts[3])) + "\""

							re = regexp.MustCompile(`^WARNING\s.*$`)
							if re.MatchString(parts[3]) {
								notify("boardRuntimeWarning", info)
							} else {
								notify("boardRuntimeError", info)
							}
						}
					}

					line = ""
				} else {
					if buffer[0] != '\r' {
						line = line + string(buffer[0])
					}
				}

				if board.consoleOut {
					ConsoleUp <- buffer[0]
				}

				if board.consoleIn {
					board.RXQueue <- buffer[0]
				}
			}
		}
	}
}

func (board *Board) attach(info *serial.Info, prerequisites bool) {
	defer func() {
		if err := recover(); err != nil {
			board.detach()
			
			vendorId, productId, _ := board.devInfo.USBVIDPID()
			if (vendorId == 0x1a86) && (productId == 0x7523) {
				connectedBoard = board
			}
		} else {
			log.Println("board attached")
		}
	}()

	log.Println("attaching board ...")

	board.devInfo = info

	// Configure options or serial port connection
	options := serial.RawOptions
	options.BitRate = 115200
	options.Mode = serial.MODE_READ_WRITE
	options.DTR = serial.DTR_OFF
	options.RTS = serial.RTS_OFF

	// Open port
	port, openErr := options.Open(info.Name())
	if openErr != nil {
		panic(openErr)
	}

	// Create board struct
	board.port = port
	board.dev = info.Name()
	board.RXQueue = make(chan byte, 10*1024)
	board.chunkSize = 255
	board.disableInspectorBootNotify = false
	board.consoleOut = true
	board.consoleIn = false
	board.quit = make(chan bool)
	board.timeoutVal = math.MaxInt32
	board.validFirmware = true

	Upgrading = false

	go board.inspector()

	// Reset the board
	board.reset(prerequisites)
	connectedBoard = board

	notify("boardAttached", "")
}

func (board *Board) detach() {
	log.Println("detaching board ...")

	// Close board
	if connectedBoard != nil {
		log.Println("closing serial port ...")

		// Close serial port
		board.port.Close()

		time.Sleep(time.Millisecond * 1000)
	}

	connectedBoard = nil
}

/*
 * Serial port primitives
 */

// Read one byte from RXQueue
func (board *Board) read() byte {
	if board.timeoutVal != math.MaxInt32 {
		for {
			select {
			case c := <-board.RXQueue:
				return c
			case <-time.After(time.Millisecond * time.Duration(board.timeoutVal)):
				panic(errors.New("timeout"))
			}
		}
	} else {
		return <-board.RXQueue
	}
}

// Read one line from RXQueue
func (board *Board) readLineCRLF() string {
	var buffer bytes.Buffer
	var b byte

	for {
		b = board.read()
		if b == '\n' {
			return buffer.String()
		} else {
			if b != '\r' {
				buffer.WriteString(string(rune(b)))
			}
		}
	}

	return ""
}

func (board *Board) readLineCR() string {
	var buffer bytes.Buffer
	var b byte

	for {
		b = board.read()
		if b == '\r' {
			return buffer.String()
		} else {
			buffer.WriteString(string(rune(b)))
		}
	}

	return ""
}

func (board *Board) consume() {
	time.Sleep(time.Millisecond * 200)

	for len(board.RXQueue) > 0 {
		board.read()
	}
}

// Wait until board is ready
func (board *Board) waitForReady() bool {
	booting := false
	whitecat := false
	failingBack := 0
	
	line := ""

	log.Println("waiting fot ready ...")

	vendorId, productId, _ := board.devInfo.USBVIDPID()
	
	board.timeout(4000)
	
	timeout := time.After(time.Millisecond * time.Duration(board.timeoutVal))
	
	for {
		select {
		case <-timeout:
			panic(errors.New("timeout"))
		default:
			line = board.readLineCRLF()

			if regexp.MustCompile(`^.*boot: Failed to verify app image.*$`).MatchString(line) {
				board.validFirmware = false
				notify("boardUpdate", "Corrupted firmware")
				return false
			}

			if regexp.MustCompile(`^.*boot: No bootable app partitions in the partition table.*$`).MatchString(line) {
				board.validFirmware = false
				notify("boardUpdate", "Corrupted firmware")
				return false
			}

			if regexp.MustCompile(`^Falling back to built-in command interpreter.$`).MatchString(line) {
				failingBack = failingBack + 1
				if (failingBack > 4) {
					board.validFirmware = false
					notify("boardUpdate", "Flash error")
					return false
				}
			}

			if !booting {
				if (vendorId == 0x1a86) && (productId == 0x7523) {
					booting = regexp.MustCompile(`Booting Lua RTOS...`).MatchString(line)
				} else {
					booting = regexp.MustCompile(`^rst:.*\(POWERON_RESET\),boot:.*(.*)$`).MatchString(line)
					if (!booting) {
						booting = regexp.MustCompile(`^rst:.*\(RTCWDT_RTC_RESET\),boot:.*(.*)$`).MatchString(line)
					}
				}
			} else {
				if !whitecat {
					if (vendorId != 0x1a86) || (productId != 0x7523) {
						whitecat = regexp.MustCompile(`Booting Lua RTOS...`).MatchString(line)
					} else {
						whitecat = true
					}
					if whitecat {
						// Send Ctrl-D
						board.port.Write([]byte{4})
					}
					board.consoleOut = true
				} else {
					if regexp.MustCompile(`^Lua RTOS-boot-scripts-aborted-ESP32$`).MatchString(line) {
						return true
					}
				}
			}
		}
	}
}


// Test if line corresponds to Lua RTOS prompt
func isPrompt(line string) bool {
	return regexp.MustCompile("^/.*>.*$").MatchString(line)
}

func (board *Board) getInfo() string {
	board.consoleOut = false
	board.consoleIn = true
	board.timeout(2000)
	info := board.sendCommand("dofile(\"/_info.lua\")")
	board.noTimeout()
	board.consoleOut = true
	board.consoleIn = false

	info = strings.Replace(info, ",}", "}", -1)
	info = strings.Replace(info, ",]", "]", -1)

	return info
}

// Send a command to the board
func (board *Board) sendCommand(command string) string {
	var response string = ""

	// Send command. We must append the \r\n chars at the end
	board.port.Write([]byte(command + "\r\n"))

	// Read response, that it must be the send command.
	line := board.readLineCRLF()
	if line == command {
		// Read until prompt
		for {
			line = board.readLineCRLF()
			log.Print(line)
			if isPrompt(line) {
				return response
			} else {
				if response != "" {
					response = response + "\r\n"
				}
				response = response + line
			}
		}
	} else {
		return ""
	}

	return ""
}

func (board *Board) reset(prerequisites bool) {
	defer func() {
		board.noTimeout()
		board.consoleOut = true
		board.consoleIn = false

		if err := recover(); err != nil {
			panic(err)
		}
	}()

	board.consume()

	board.consoleOut = false
	board.consoleIn = true

	// Reset board
	options := serial.RawOptions
	options.BitRate = 115200
	options.Mode = serial.MODE_READ_WRITE

	options.RTS = serial.RTS_OFF
	board.port.Apply(&options)

	time.Sleep(time.Millisecond * 10)

	options.RTS = serial.RTS_ON
	board.port.Apply(&options)

	time.Sleep(time.Millisecond * 10)

	options.RTS = serial.RTS_OFF
	board.port.Apply(&options)

	if !board.waitForReady() {
		return
	}

	board.consume()

	log.Println("board is ready ...")

	if prerequisites && (connectedBoard != nil) {
		notify("boardUpdate", "Downloading prerequisites")

		// Clean
		os.RemoveAll(path.Join(AppDataTmpFolder, "*"))

		// Upgrade prerequisites
		resp, err := http.Get("https://ide.whitecatboard.org/boards/prerequisites.zip")
		if err == nil {
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				err = ioutil.WriteFile(path.Join(AppDataTmpFolder, "prerequisites.zip"), body, 0777)
				if err == nil {
					unzip(path.Join(AppDataTmpFolder, "prerequisites.zip"), path.Join(AppDataTmpFolder, "prerequisites_files"))
				} else {
					panic(err)
				}
			} else {
				panic(err)
			}
		} else {
			panic(err)
		}

		notify("boardUpdate", "Uploading framework")

		board.consoleOut = false
		board.consoleIn = true

		// Test for lib/lua
		board.timeout(1000)
		exists := board.sendCommand("do local att = io.attributes(\"/lib\"); print(att ~= nil and att.type == \"directory\"); end")
		if exists != "true" {
			log.Println("creating /lib folder")
			board.sendCommand("os.mkdir(\"/lib\")")
		} else {
			log.Println("/lib folder, present")
		}

		exists = board.sendCommand("do local att = io.attributes(\"/lib/lua\"); print(att ~= nil and att.type == \"directory\"); end")
		if exists != "true" {
			log.Println("creating /lib/lua folder")
			board.sendCommand("os.mkdir(\"/lib/lua\")")
		} else {
			log.Println("/lib/lua folder, present")
		}
		board.noTimeout()

		buffer, err := ioutil.ReadFile(path.Join(AppDataTmpFolder, "prerequisites_files", "lua", "board-info.lua"))
		if err == nil {
			resp := board.writeFile("/_info.lua", buffer)
			if resp == "" {
				panic(errors.New("timeout"))
			}
		} else {
			panic(err)
		}

		files, err := ioutil.ReadDir(path.Join(AppDataTmpFolder, "prerequisites_files", "lua", "lib"))
		if err == nil {
			for _, finfo := range files {
				if regexp.MustCompile(`.*\.lua`).MatchString(finfo.Name()) {
					file, _ := ioutil.ReadFile(path.Join(AppDataTmpFolder, "prerequisites_files", "lua", "lib", finfo.Name()))
					log.Println("Sending ", "/lib/lua/"+finfo.Name(), " ...")
					resp := board.writeFile("/lib/lua/"+finfo.Name(), file)
					if resp == "" {
						panic(errors.New("timeout"))
					}
					board.consume()
				}
			}
		} else {
			panic(err)
		}

		board.consoleOut = true

		// Get board info
		info := board.getInfo()

		// Parse some board info
		var boardInfo BoardInfo

		json.Unmarshal([]byte(info), &boardInfo)

		// Test for a newer software build
		board.newBuild = false

		resp, err = http.Get(LastBuildURL + "?board=" + board.firmware)
		if err == nil {
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				lastCommit := string(body)

				if boardInfo.Commit != lastCommit {
					board.newBuild = true
					log.Println("new firmware available: ", lastCommit)
				}
			} else {
				panic(err)
			}
		} else {
			panic(err)
		}

		board.info = info
		board.model = boardInfo.Board
		board.subtype = boardInfo.Subtype
		board.brand = boardInfo.Brand
		board.ota = boardInfo.Ota

		firmware := ""

		if board.brand != "" {
			firmware = board.brand + "-"
		}

		firmware = firmware + board.model

		if board.subtype != "" {
			firmware = firmware + "-" + board.subtype
		}

		board.firmware = firmware
	}
}

func (board *Board) getDirContent(path string) string {
	var content string

	defer func() {
		board.noTimeout()
		board.consoleOut = true
		board.consoleIn = false

		if err := recover(); err != nil {
		}
	}()

	content = ""

	board.consoleOut = false
	board.consoleIn = true

	board.timeout(1000)
	response := board.sendCommand("os.ls(\"" + path + "\")")
	for _, line := range strings.Split(response, "\n") {
		element := strings.Split(strings.Replace(line, "\r", "", -1), "\t")

		if len(element) == 4 {
			if content != "" {
				content = content + ","
			}

			content = content + "{" +
				"\"type\": \"" + element[0] + "\"," +
				"\"size\": \"" + element[1] + "\"," +
				"\"date\": \"" + element[2] + "\"," +
				"\"name\": \"" + element[3] + "\"" +
				"}"
		}
	}

	board.consoleOut = true

	return "[" + content + "]"
}

func (board *Board) writeFile(path string, buffer []byte) string {
	defer func() {
		board.noTimeout()
		board.consoleOut = true
		board.consoleIn = false

		if err := recover(); err != nil {
		}
	}()

	board.timeout(2000)
	board.consoleOut = false
	board.consoleIn = true

	writeCommand := "io.receive(\"" + path + "\")"

	outLen := 0
	outIndex := 0

	board.consume()

	notify("progress", "\033[K"+strconv.Itoa(outIndex)+" of "+strconv.Itoa(len(buffer))+" bytes sended ("+path+") ...\r")

	// Send command and test for echo
	board.port.Write([]byte(writeCommand + "\r"))
	if board.readLineCR() == writeCommand {
		for {
			// Wait for chunk
			if board.readLineCRLF() == "C" {
				// Get chunk length
				if outIndex < len(buffer) {
					if outIndex+board.chunkSize < len(buffer) {
						outLen = board.chunkSize
					} else {
						outLen = len(buffer) - outIndex
					}
				} else {
					outLen = 0
				}

				notify("progress", "\033[K"+strconv.Itoa(outIndex+outLen)+" of "+strconv.Itoa(len(buffer))+" bytes sended ("+path+") ...\r")

				// Send chunk length
				board.port.Write([]byte{byte(outLen)})

				if outLen > 0 {
					// Send chunk
					board.port.Write(buffer[outIndex : outIndex+outLen])
				} else {
					break
				}

				outIndex = outIndex + outLen
			}
		}

		if board.readLineCRLF() == "true" {
			board.consume()

			notify("progress", "\033[Kfile sended\r")

			return "ok"
		}
	}

	return ""
}

func (board *Board) runCode(buffer []byte) {
	writeCommand := "os.run()"

	outLen := 0
	outIndex := 0

	board.consoleOut = false
	board.consoleIn = true

	// Send command
	board.port.Write([]byte(writeCommand + "\r"))
	for {
		// Wait for chunk
		if board.readLineCRLF() == "C" {
			// Get chunk length
			if outIndex < len(buffer) {
				if outIndex+board.chunkSize < len(buffer) {
					outLen = board.chunkSize
				} else {
					outLen = len(buffer) - outIndex
				}
			} else {
				outLen = 0
			}

			// Send chunk length
			board.port.Write([]byte{byte(outLen)})

			if outLen > 0 {
				// Send chunk
				board.port.Write(buffer[outIndex : outIndex+outLen])
			} else {
				break
			}

			outIndex = outIndex + outLen
		}
	}

	board.consume()

	board.consoleOut = true
	board.consoleOut = false
}

func (board *Board) readFile(path string) []byte {
	defer func() {
		board.noTimeout()
		board.consoleOut = true
		board.consoleIn = false

		if err := recover(); err != nil {
		}
	}()

	var buffer bytes.Buffer
	var inLen byte
	var received int

	board.timeout(2000)
	board.consoleOut = false
	board.consoleIn = true

	received = 0

	// Command for read file
	readCommand := "io.send(\"" + path + "\")"

	notify("progress", "\033[K"+strconv.Itoa(received)+" bytes received ...\r")

	// Send command and test for echo
	board.port.Write([]byte(readCommand + "\r"))
	if board.readLineCRLF() == readCommand {
		for {
			// Wait for chunk
			board.port.Write([]byte("C\n"))

			// Read chunk size
			inLen = board.read()

			// Read chunk
			if inLen > 0 {
				for inLen > 0 {
					buffer.WriteByte(board.read())

					inLen = inLen - 1
					received = received + 1
				}
			} else {
				// No more data
				break
			}

			notify("progress", "\033[K"+strconv.Itoa(received)+" bytes received ...\r")
		}

		board.consume()

		notify("progress", "\n\033[Kfile received\r\n")

		return buffer.Bytes()
	}

	return nil
}

func (board *Board) runProgram(path string, code []byte) {
	board.disableInspectorBootNotify = true

	board.consoleOut = false

	// Reset board
	board.reset(false)
	board.disableInspectorBootNotify = false

	board.consoleOut = false
	board.consoleIn = true

	// First update autorun.lua, which run the target file
	board.writeFile("/autorun.lua", []byte("dofile(\""+path+"\")\r\n"))

	// Now write code to target file
	board.writeFile(path, code)

	// Run the target file
	board.port.Write([]byte("require(\"block\");wcBlock.delevepMode=true;dofile(\"" + path + "\")\r"))

	board.consume()

	board.consoleOut = true
	board.consoleIn = false
}

func (board *Board) runCommand(code []byte) string {
	board.consoleOut = false
	board.consoleIn = true
	result := board.sendCommand(string(code))
	board.consume()
	board.consoleOut = true
	board.consoleIn = false

	return result
}

func exec_cmd(cmd string, wg *sync.WaitGroup) {
	fmt.Println(cmd)
	out, err := exec.Command(cmd).Output()
	if err != nil {
		fmt.Println("error occured")
		fmt.Printf("%s\n", err)
	}
	fmt.Printf("%s", out)
	wg.Done()
}

func (board *Board) upgrade(erase bool, flash bool, flashFS bool) {
	var boardName string
	var out string = ""

	Upgrading = true

	// First detach board for free serial port
	board.detach()

	// Download tool for flashing
	err := downloadEsptool()
	if err != nil {
		notify("boardUpdate", err.Error())
		time.Sleep(time.Millisecond * 1000)
		Upgrading = false
		
		return
	}

	if erase {
		notify("progress", "Erasing flash")

		flash_args := "--chip esp32 --port " + board.dev + " --baud 115200 erase_flash"

		// Build the flash command
		cmdArgs := regexp.MustCompile(`'.*?'|".*?"|\S+`).FindAllString(flash_args, -1)

		for i, _ := range cmdArgs {
			cmdArgs[i] = strings.Replace(cmdArgs[i], "\"", "", -1)
		}

		// Prepare for execution
		cmd := exec.Command(AppDataTmpFolder+"/utils/esptool/esptool", cmdArgs...)

		log.Println("executing: ", "\""+AppDataTmpFolder+"/utils/esptool/esptool\"")

		// We need to read command stdout for show the progress in the IDE
		stdout, _ := cmd.StdoutPipe()

		notify("progress", "\r                   \r")

		// Start
		cmd.Start()

		// Read stdout until EOF
		c := make([]byte, 1)
		for {
			_, err := stdout.Read(c)
			if err != nil {
				break
			}

			if c[0] == '\r' || c[0] == '\n' {
				out = strings.Replace(out, "...", "", -1)
				if out != "" {
					notify("progress", "Erasing flash ...\r")
				}
				out = ""
			} else {
				out = out + string(c)
			}

		}

		log.Println("Erased")
	}

	if flash || flashFS {
		// Download firmware
		err = downloadFirmware(board.firmware)
		if err != nil {
			notify("boardUpdate", err.Error())
			time.Sleep(time.Millisecond * 1000)
			Upgrading = false
			
			return
		}

		// Get the board name part of the firmware files for
		// current board model
		boardName = board.getFirmwareName()
	}

	if flash {
		notify("progress", "Flasing last firmware\r\n")

		// Read flash arguments
		b, err := ioutil.ReadFile(AppDataTmpFolder + "/firmware_files/flash_args")
		if err != nil {
			notify("boardUpdate", err.Error())
			time.Sleep(time.Millisecond * 1000)
			Upgrading = false
			
			return
		}

		flash_args := string(b)

		flash_args = strings.Replace(flash_args, "bootloader."+boardName+".bin", "\""+AppDataTmpFolder+"/firmware_files/bootloader."+boardName+".bin\"", -1)
		flash_args = strings.Replace(flash_args, "lua_rtos."+boardName+".bin", "\""+AppDataTmpFolder+"/firmware_files/lua_rtos."+boardName+".bin\"", -1)
		flash_args = strings.Replace(flash_args, "partitions_singleapp."+boardName+".bin", "\""+AppDataTmpFolder+"/firmware_files/partitions_singleapp."+boardName+".bin\"", -1)
		flash_args = strings.Replace(flash_args, "partitions-ota."+boardName+".bin", "\""+AppDataTmpFolder+"/firmware_files/partitions_singleapp."+boardName+".bin\"", -1)
		flash_args = strings.Replace(flash_args, "partitions-ota.bin", "\""+AppDataTmpFolder+"/firmware_files/partitions_singleapp."+boardName+".bin\"", -1)
		flash_args = strings.Replace(flash_args, "phy_init_data."+boardName+".bin", "\""+AppDataTmpFolder+"/firmware_files/phy_init_data."+boardName+".bin\"", -1)
		flash_args = strings.Replace(flash_args, "phy_init_data.bin", "\""+AppDataTmpFolder+"/firmware_files/phy_init_data."+boardName+".bin\"", -1)

		// Add usb port to flash arguments
		flash_args = "--port " + board.dev + " " + flash_args

		log.Println("flash args: ", flash_args)

		// Build the flash command
		cmdArgs := regexp.MustCompile(`'.*?'|".*?"|\S+`).FindAllString(flash_args, -1)

		for i, _ := range cmdArgs {
			cmdArgs[i] = strings.Replace(cmdArgs[i], "\"", "", -1)
		}

		// Prepare for execution
		cmd := exec.Command(AppDataTmpFolder+"/utils/esptool/esptool", cmdArgs...)

		log.Println("executing: ", "\""+AppDataTmpFolder+"/utils/esptool/esptool\"")

		// We need to read command stdout for show the progress in the IDE
		stdout, _ := cmd.StdoutPipe()

		// Start
		cmd.Start()

		// Read stdout until EOF
		c := make([]byte, 1)
		for {
			_, err := stdout.Read(c)
			if err != nil {
				break
			}

			if c[0] == '\r' || c[0] == '\n' {
				out = strings.Replace(out, "...", "", -1)
				if out != "" {
					notify("boardUpdate", out)
				}
				out = ""
			} else {
				out = out + string(c)
			}

		}

		log.Println("Upgraded")
	}

	if flashFS {
		notify("progress", "Flasing last file system\r\n")

		// Read flash arguments
		b, err := ioutil.ReadFile(AppDataTmpFolder + "/firmware_files/flashfs_args")
		if err != nil {
			notify("boardUpdate", err.Error())
			time.Sleep(time.Millisecond * 1000)
			Upgrading = false
			
			return
		}

		flash_args := string(b)

		flash_args = strings.Replace(flash_args, "spiffs_image."+boardName+".bin", "\""+AppDataTmpFolder+"/firmware_files/spiffs_image."+boardName+".bin\"", -1)

		// Add usb port to flash arguments
		flash_args = "--port " + board.dev + " " + flash_args

		log.Println("flash args: ", flash_args)

		// Build the flash command
		cmdArgs := regexp.MustCompile(`'.*?'|".*?"|\S+`).FindAllString(flash_args, -1)

		for i, _ := range cmdArgs {
			cmdArgs[i] = strings.Replace(cmdArgs[i], "\"", "", -1)
		}

		// Prepare for execution
		cmd := exec.Command(AppDataTmpFolder+"/utils/esptool/esptool", cmdArgs...)

		log.Println("executing: ", "\""+AppDataTmpFolder+"/utils/esptool/esptool\"")

		// We need to read command stdout for show the progress in the IDE
		stdout, _ := cmd.StdoutPipe()

		// Start
		cmd.Start()

		// Read stdout until EOF
		c := make([]byte, 1)
		for {
			_, err := stdout.Read(c)
			if err != nil {
				break
			}

			if c[0] == '\r' || c[0] == '\n' {
				out = strings.Replace(out, "...", "", -1)
				if out != "" {
					notify("boardUpdate", out)
				}
				out = ""
			} else {
				out = out + string(c)
			}

		}

		log.Println("Upgraded")
	}

	time.Sleep(time.Millisecond * 1000)
	Upgrading = false
}

func (board *Board) selectSupportedBoard() {
	var supportedBoards SupportedBoards

	okayBoards := []string{}

	board.brand = ""
	board.subtype = ""

	// Get supported boards
	resp, err := http.Get(SupportedBoardsURL)
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &supportedBoards)

			fmt.Println("\nPlease, enter your board type:\n")

			for option, supportedBoard := range supportedBoards {
				okayBoards = append(okayBoards, strconv.Itoa(option+1))
				fmt.Printf("% 3d: %s\n", option+1, supportedBoard.Description)
			}
		} else {
			panic(err)
		}
	} else {
		panic(err)
	}

	fmt.Print("\nType: ")

	selectedBoard := ""

	_, err = fmt.Scanln(&selectedBoard)
	if err == nil {
		if containsString(okayBoards, selectedBoard) {
			selectedBoard, _ := strconv.Atoi(selectedBoard)

			for option, supportedBoard := range supportedBoards {
				if option == selectedBoard-1 {
					board.model = supportedBoard.Type
					board.subtype = supportedBoard.Subtype
				}
			}

			firmware := ""

			if board.brand != "" {
				firmware = board.brand + "-"
			}

			firmware = firmware + board.model

			if board.subtype != "" {
				firmware = firmware + "-" + board.subtype
			}

			board.firmware = firmware
		}
	}
}

func (board *Board) getFirmwareName() string {
	var supportedBoards SupportedBoards

	// Get supported boards
	resp, err := http.Get(SupportedBoardsURL)
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &supportedBoards)

			for _, supportedBoard := range supportedBoards {
				if (supportedBoard.Type == board.model) && (supportedBoard.Subtype == board.subtype) {
					firmware := ""

					if supportedBoard.Brand != "" {
						firmware = supportedBoard.Brand + "-"
					}

					firmware = firmware + supportedBoard.Id

					if supportedBoard.Subtype != "" {
						firmware = firmware + "-" + supportedBoard.Subtype
					}

					return firmware
				}
			}
		} else {
			panic(err)
		}
	} else {
		panic(err)
	}

	return ""
}
