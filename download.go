/*
 * Whitecat Blocky Environment, download abstraction
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
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0777)
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}

			err = os.MkdirAll(fdir, 0777)
			if err != nil {
				log.Fatal(err)
				return err
			}
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

func downloadEsptool() error {
	notify("boardUpdate", "Downloading esptool")

	url := "http://downloads.whitecatboard.org/esptool/esptool-" + runtime.GOOS + ".zip"

	log.Println("downloading esptool from " + url + " ...")

	resp, err := http.Get(url)
	if err == nil {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			log.Println("downloaded")

			err = ioutil.WriteFile(path.Join(AppDataTmpFolder, "esptool.zip"), body, 0777)
			if err == nil {
				notify("boardUpdate", "Unpacking esptool")

				log.Println("unpacking esptool ...")

				unzip(path.Join(AppDataTmpFolder, "esptool.zip"), path.Join(AppDataTmpFolder, "utils"))
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		return err
	}
	
	return nil
}

func downloadFirmware(model string) error {
	notify("boardUpdate", "Downloading firmware")

	url := "http://whitecatboard.org/firmware.php?board=" + model

	log.Println("downloading firmware from " + url + "...")

	resp, err := http.Get(url)
	if err == nil {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			err = ioutil.WriteFile(path.Join(AppDataTmpFolder, "firmware.zip"), body, 0777)
			if err == nil {
				notify("boardUpdate", "Unpacking firmware")

				log.Println("unpacking firmware ...")

				unzip(path.Join(AppDataTmpFolder, "firmware.zip"), path.Join(AppDataTmpFolder, "firmware_files"))
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		return err
	}
	
	return nil
}
