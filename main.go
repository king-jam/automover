package main

import (
	"fmt"
	"time"

	"github.com/go-vgo/robotgo"
)

func main() {
	offset := 20
	var x, y int
	x, y = robotgo.GetMousePos()
	for {

		fmt.Println("mouse coordinates: x=", x+offset, ", y=", y+offset)
		robotgo.MoveMouse(x+offset, y+offset)
		time.Sleep(10 * time.Second)
		robotgo.MoveMouse(x-offset, y-offset)
		fmt.Println("mouse coordinates: x=", x-offset, ", y=", y-offset)
		time.Sleep(10 * time.Second)
	}
}
