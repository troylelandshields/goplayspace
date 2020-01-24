package main

import (
	"github.com/gopherjs/vecty"
	"github.com/iafan/goplayspace/client/component/app"
	"github.com/iafan/goplayspace/client/component/drawboard"
	"github.com/iafan/goplayspace/client/draw"
)

func main() {
	vecty.SetTitle("Gophers")

	actions := draw.New([]string{houseStr, squaresStr, "drawing mode"})

	a := &app.Application{
		DrawBoard: drawboard.New(actions),
	}

	vecty.RenderBody(a)
}

const houseStr = `draw mode

// draw the roof
say Building the roof
color red
right 30
forward 5
right 120
forward 5
right 30

// draw the walls
say Building the walls
color black
forward 5
right
forward 5
right
forward 5
right
forward 5
right

// move to the door start
color off
forward 5
right
forward
right

// draw the door
say Building the door
color green
forward 2
left
forward
left
forward 2
left
forward

// move away from the house
color off
forward 3
left
say Done!`

const squaresStr = `draw mode
		
say Let's start...
right 18
color red

forward 7
say One...
right 144

forward 7
say Two...
right 144

forward 7
say Three...
right 144

forward 7
say Four...
right 144

forward 7
say We've got a star!
right 144`
