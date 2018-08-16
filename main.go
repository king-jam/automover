package main

import (
	"fmt"
	"time"

	"github.com/go-vgo/robotgo"
)

func main() {
	var x, y int

	for {
		x, y = robotgo.GetMousePos()
		fmt.Println("mouse coordinates: x=", x, ", y=", y)
		robotgo.MoveMouse(x+1, y+1)
		time.Sleep(30 * time.Second)
		robotgo.MoveMouse(x-1, y-1)
		time.Sleep(30 * time.Second)
	}
}
