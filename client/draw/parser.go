package draw

import (
	"regexp"
	"strconv"
	"strings"
)

const cmdStartDrawMode = "draw mode"

var cmdForwardR = regexp.MustCompile(`^forward$`)
var cmdForwardNR = regexp.MustCompile(`^forward (\d+(\.\d+)?)$`)
var cmdLeftR = regexp.MustCompile(`^left$`)
var cmdLeftNR = regexp.MustCompile(`^left (\d+(\.\d+)?)$`)
var cmdRightR = regexp.MustCompile(`^right$`)
var cmdRightNR = regexp.MustCompile(`^right (\d+(\.\d+)?)$`)
var cmdColorOffR = regexp.MustCompile(`^(?:color|colour) off$`)
var cmdColorSR = regexp.MustCompile(`^(?:color|colour) (.+)$`)
var cmdWidthNR = regexp.MustCompile(`^width (\d+(\.\d+)?)$`)
var cmdSaySR = regexp.MustCompile(`^say (.+)$`)

const (
	Step = iota
	Left
	Right
	Color
	Width
	Say
)

type Action struct {
	// Actor int //TODO:get this to work
	Cmd  string
	Kind int
	FVal float64
	SVal string
}

type Actor interface {
	ID() string
	Next() (*Action, bool)
}

type ActorsList interface {
	Actors() []Actor
}

func New(instructions []string) ActorsList {
	var actors []Actor

	for i, s := range instructions {
		actor := parseString(strconv.Itoa(i), s)
		converted := Actor(actor)

		actors = append(actors, converted)
	}

	return &SimpleActorList{
		actors: actors,
	}
}

var _ Actor = &SimpleActor{}

type SimpleActor struct {
	id           string
	currentIndex int
	actions      []*Action
}

type SimpleActorList struct {
	actors []Actor
}

func (s *SimpleActorList) Actors() []Actor {
	return s.actors
}

func (s *SimpleActor) Next() (*Action, bool) {
	if len(s.actions) <= s.currentIndex {
		return nil, false
	}

	nextActions := s.actions[s.currentIndex]

	s.currentIndex = s.currentIndex + 1

	return nextActions, true
}

func (s *SimpleActor) ID() string {
	return s.id
}

func parseLines(id string, lines []string) *SimpleActor {
	var a []*Action

	isDrawMode := false

	for _, line := range lines {
		line = strings.ToLower(strings.TrimSpace(line))

		if !isDrawMode && line != cmdStartDrawMode {
			continue
		}
		isDrawMode = true

		if matches := cmdForwardR.FindAllStringSubmatch(line, -1); matches != nil {
			a = append(a, &Action{line, Step, 1, ""})
			continue
		}

		if matches := cmdForwardNR.FindAllStringSubmatch(line, -1); matches != nil {
			n, _ := strconv.ParseFloat(matches[0][1], 64)
			a = append(a, &Action{line, Step, n, ""})
			continue
		}

		if matches := cmdLeftR.FindAllStringSubmatch(line, -1); matches != nil {
			a = append(a, &Action{line, Left, 90, ""})
			continue
		}

		if matches := cmdLeftNR.FindAllStringSubmatch(line, -1); matches != nil {
			n, _ := strconv.ParseFloat(matches[0][1], 64)
			a = append(a, &Action{line, Left, n, ""})
			continue
		}

		if matches := cmdRightR.FindAllStringSubmatch(line, -1); matches != nil {
			a = append(a, &Action{line, Right, 90, ""})
			continue
		}

		if matches := cmdRightNR.FindAllStringSubmatch(line, -1); matches != nil {
			n, _ := strconv.ParseFloat(matches[0][1], 64)
			a = append(a, &Action{line, Right, n, ""})
			continue
		}

		if matches := cmdColorOffR.FindAllStringSubmatch(line, -1); matches != nil {
			a = append(a, &Action{line, Color, 0, ""})
			continue
		}

		if matches := cmdColorSR.FindAllStringSubmatch(line, -1); matches != nil {
			a = append(a, &Action{line, Color, 0, matches[0][1]})
			continue
		}

		if matches := cmdWidthNR.FindAllStringSubmatch(line, -1); matches != nil {
			n, _ := strconv.ParseFloat(matches[0][1], 64)
			a = append(a, &Action{line, Width, n, ""})
			continue
		}

		if matches := cmdSaySR.FindAllStringSubmatch(line, -1); matches != nil {
			a = append(a, &Action{line, Say, 0, matches[0][1]})
			continue
		}

	}

	return &SimpleActor{
		id:      id,
		actions: a,
	}
}

func parseString(id string, s string) *SimpleActor {
	return parseLines(id, strings.Split(s, "\n"))
}
