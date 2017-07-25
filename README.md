# What's The Whitecat Create Agent?

The Whitecat Create Agent is a small piece of software that runs on your computer, and allows the communication beetween a [Lua RTOS device](https://github.com/whitecatboard/Lua-RTOS-ESP32) and [The Whitecat IDE](https://github.com/whitecatboard/whitecat-ide). This is needed because the IDE must perform many operations that needs to use your computer hardware, and the IDE is on the cloud!. The communication beetween the agent and the IDE is made using websockets

# How to build?

1. Go to your Go's workspace location

   For example:

   ```lua
   cd gows
   ```

1. Download and install

   ```lua
   go get github.com/whitecatboard/whitecat-create-agent
   ```

1. Go to the project source root

   ```lua
   cd src/github.com/whitecatboard/whitecat-create-agent
   ```

1. Build project

   ```lua
   go build
   ```
   
   For execute:
   
   Linux / OSX:
   
   ```lua
   ./whitecat-create-agent
   ```
   
   Windows:
   
   ```lua
   whitecat-create-agent.exe
   ```

# Read the wiki

You can find more informatio about The Whitecat Create Agent in our [wiki](https://github.com/whitecatboard/whitecat-create-agent/wiki).
