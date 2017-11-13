<?php
	$firmwares["N1ESP32"] = "WHITECAT-ESP32-N1";
	$firmwares["ESP32THING"] = "ESP32-THING";
	$firmwares["ESP32COREBOARD"] = "CORE-BOARD";
	$firmwares["GENERIC"] = "GENERIC";

	$firmwares["N1ESP32-OTA"] = "WHITECAT-ESP32-N1-OTA";
	$firmwares["ESP32THING-OTA"] = "ESP32-THING-OTA";
	$firmwares["ESP32COREBOARD-OTA"] = "CORE-BOARD-OTA";
	$firmwares["GENERIC-OTA"] = "GENERIC-OTA";

	$firmware = $_GET["firmware"];

	if ($firmware == "") {
		die();
	}

	$firmware = $firmwares[$firmware];
	
	if ($firmware == "") {
		die();
	}

	// Dir all files for firmware
	$file = glob("/home/whitecatboard/www/lua-rtos-builds/*$firmware*");
	
	// Get last file
	$last = count($file) - 1;

	header('Content-Type: application/zip');
	header('Content-Disposition: attachment; filename="'.basename($file[$last]).'"');
	header('Content-Length: '.filesize($file[$last]));
	readfile($file[$last]);		
?>