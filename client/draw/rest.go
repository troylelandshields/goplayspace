package draw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func NewHTTPActorsList(addr string) ActorsList {

	return &HTTPActorList{
		addr: addr,
	}
}

var _ Actor = &HTTPActor{}

type move struct {
	Description string
}

type HTTPActor struct {
	addr  string
	moves []move

	ArtistID string `json:"ID"`
	Name     string
}

type HTTPActorList struct {
	actors []HTTPActor
	addr   string
}

func (s *HTTPActorList) Actors() []Actor {
	resp, err := http.Get(s.addr + "/api/artists")
	if err != nil {
		fmt.Println("Err getting artists", err)
		return nil
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&s.actors)
	if err != nil {
		fmt.Println("Err decoding artists", err)
		return nil
	}

	actors := make([]Actor, len(s.actors))
	for i, a := range s.actors {
		s.actors[i].addr = s.addr
		actors[i] = Actor(&a)
	}

	return actors
}

func (s *HTTPActor) Next() (*Action, bool) {

	// get more moves if we're out
	if len(s.moves) == 0 {
		resp, err := http.Get(s.addr + "/api/artists/" + s.ArtistID + "/moves")
		if err != nil {
			fmt.Println("Err getting moves", err)
			return nil, false
		}

		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&s.moves)
		if err != nil {
			fmt.Println("Err decoding moves", err)
			return nil, false
		}
	}

	if len(s.moves) > 0 {
		action := parseDescription(s.moves[0].Description)
		s.moves = s.moves[1:len(s.moves)]
		return action, true
	}

	return nil, false
}

func (s *HTTPActor) ID() string {
	return s.ArtistID
}

func parseDescription(line string) *Action {
	var a *Action

	line = strings.ToLower(strings.TrimSpace(line))

	if matches := cmdForwardR.FindAllStringSubmatch(line, -1); matches != nil {
		a = &Action{line, Step, 1, ""}
	}

	if matches := cmdForwardNR.FindAllStringSubmatch(line, -1); matches != nil {
		n, _ := strconv.ParseFloat(matches[0][1], 64)
		a = &Action{line, Step, n, ""}
	}

	if matches := cmdLeftR.FindAllStringSubmatch(line, -1); matches != nil {
		a = &Action{line, Left, 90, ""}
	}

	if matches := cmdLeftNR.FindAllStringSubmatch(line, -1); matches != nil {
		n, _ := strconv.ParseFloat(matches[0][1], 64)
		a = &Action{line, Left, n, ""}
	}

	if matches := cmdRightR.FindAllStringSubmatch(line, -1); matches != nil {
		a = &Action{line, Right, 90, ""}
	}

	if matches := cmdRightNR.FindAllStringSubmatch(line, -1); matches != nil {
		n, _ := strconv.ParseFloat(matches[0][1], 64)
		a = &Action{line, Right, n, ""}
	}

	if matches := cmdColorOffR.FindAllStringSubmatch(line, -1); matches != nil {
		a = &Action{line, Color, 0, ""}
	}

	if matches := cmdColorSR.FindAllStringSubmatch(line, -1); matches != nil {
		a = &Action{line, Color, 0, matches[0][1]}
	}

	if matches := cmdWidthNR.FindAllStringSubmatch(line, -1); matches != nil {
		n, _ := strconv.ParseFloat(matches[0][1], 64)
		a = &Action{line, Width, n, ""}
	}

	if matches := cmdSaySR.FindAllStringSubmatch(line, -1); matches != nil {
		a = &Action{line, Say, 0, matches[0][1]}
	}

	return a
}
