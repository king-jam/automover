# automover

Fun little project to move a mouse around. Trying to prove how much open source can help a project get done quickly.

## Important Warning

```
Note: Cannot Vendor this due to amount of C/C++ Source code required. Go Modules do not copy .c/.cpp files.
```

### Cross-Compiling for Windows (on Ubuntu 18.04)
Pre-Reqs
```
sudo apt install gcc libc6-dev
sudo apt install gcc-mingw-w64-x86-64 libz-mingw-w64-dev
sudo apt install libx11-dev xorg-dev libxtst-dev libpng++-dev
sudo apt install xcb libxcb-xkb-dev x11-xkb-utils libx11-xcb-dev libxkbcommon-x11-dev
```
Compile Command
```
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build
```