# What's The Whitecat Console?

The Whitecat Console is a command line tool that allows the programmer to send and receive files to / from Lua RTOS compatible boards without using an IDE.

# How to build?

1. Go to your Go's workspace location

   For example:

   ```lua
   cd gows
   ```

1. Download and install

   ```lua
   go get github.com/whitecatboard/whitecat-console
   ```

1. Go to the project source root

   ```lua
   cd src/github.com/whitecatboard/whitecat-console
   ```

1. Build project

   ```lua
   go build
   ```
   
   For execute:
   
   Linux / OSX:
   
   ```lua
   ./wcc
   ```
   
   Windows:
   
   ```lua
   wcc.exe
   ```

# Prerequisites

Please note you need probably to download and install drivers for your board's USB-TO-SERIAL adapter for Windows and Mac OSX versions. The GNU/Linux version usually doesn't need any drivers. This drivers are required for connect to your board through a serial port connection.

   | Board              |
   |--------------------|
   | [WHITECAT ESP32 N1](https://www.silabs.com/products/development-tools/software/usb-to-uart-bridge-vcp-drivers)  | 
   | [ESP32 CORE](https://www.silabs.com/products/development-tools/software/usb-to-uart-bridge-vcp-drivers)  | 
   | [ESP32 THING](http://www.ftdichip.com/Drivers/VCP.htm)  | 


# Examples

List files in /examples directory
```lua
./wcc -p /dev/tty.SLAB_USBtoUART -ls /examples
```

Download system.lua file and store it as s.lua in your computer
```lua
./wcc -p /dev/tty.SLAB_USBtoUART -down system.lua s.lua
```

Upload s.lua file and store it as system.lua in your board
```lua
./wcc -p /dev/tty.SLAB_USBtoUART -up s.lua system.lua
```
