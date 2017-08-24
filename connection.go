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
	"github.com/mikepb/go-serial"
	"log"
)

// This channel is used by the create agent for send console output
var ConsoleUp chan byte

// Connected board
var connectedBoard *Board = nil

// This function consumes all chars in ConsoleUp channel.
// This is need for minimize changes in Whitecat Create Agent sources.
func console() {
	for {
		<-ConsoleUp
	}
}

func connect(port string) {
	log.Println("connecting to board on", port, "...")

	ConsoleUp = make(chan byte, 1024)

	go console()

	// Open port
	info, err := serial.PortByName(port)
	if err != nil {
		return
	}

	// Create a candidate board
	var candidate Board

	// Attach candidate
	candidate.attach(info, false)

	if connectedBoard != nil {
		connectedBoard.port.Write([]byte("os.shell(false)\r\n"))
		connectedBoard.consume()
	}
	
	return
}

func list_ports() {
	// Enumerate all serial ports
	ports, err := serial.ListPorts()
	if err != nil {
		return
	}
	
	for _, info := range ports {
		fmt.Println(info.Name())
	}
}
