package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LinkStatus represents whether a link is available or not.
type LinkStatus string

const (
	// UP represents a link that is available.
	UP LinkStatus = "UP"

	// DOWN represents a link that is unavailable.
	DOWN = "DOWN"
)

// LinkState represents a link's state at a given moment in time.
type LinkState struct {
	// time is the moment in time, inclusive, this state becomes valid.
	time int

	// status is the status of the link.
	status LinkStatus

	// fromNode is the source Node id.
	fromNode NodeID

	// toNode is the destination Node id.
	toNode NodeID
}

func (l *LinkState) String() string {
	return fmt.Sprintf("%d %s %d %d", l.time, l.status, l.fromNode, l.toNode)
}

func parseLinkState(state string) (*LinkState, error) {
	ls := &LinkState{}

	// Basic validation
	splitState := strings.Split(state, " ")
	if len(splitState) != 4 {
		return nil, ErrParseLinkState{msg: "must be of the form: '{TIME} {UP | DOWN} {LABEL} {LABEL}'"}
	}

	// Parse time
	time, err := strconv.Atoi(splitState[0])
	if err != nil {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("time is not an integer: '%s'", splitState[0])}
	}
	if time < 0 {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("time must be greater than 0: '%s'", splitState[0])}
	}
	ls.time = time

	// Parse status
	switch LinkStatus(splitState[1]) {
	case UP:
		ls.status = UP
	case DOWN:
		ls.status = DOWN
	default:
		return nil, ErrParseLinkState{msg: fmt.Sprintf("invalid status: '%s': must be {UP | DOWN}", splitState[1])}
	}

	// Parse labels
	lre := regexp.MustCompile(`^\d$`)
	if !lre.Match([]byte(splitState[2])) {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("invalid id: '%s': must be '^[0-9]$'", splitState[2])}
	}
	if !lre.Match([]byte(splitState[3])) {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("invalid id: '%s': must be '^[0-9]$'", splitState[3])}
	}

	// Already ensured the string represents an integer from the regex.
	rawLabel, _ := strconv.Atoi(splitState[2])
	ls.fromNode = NodeID(rawLabel)

	rawLabel, _ = strconv.Atoi(splitState[3])
	ls.toNode = NodeID(rawLabel)

	return ls, nil
}

type Link struct {
	// fromNode is the source Node id.
	fromNode NodeID

	// toNode is the destination Node id.
	toNode NodeID

	states []LinkState
}

// isUp determines whether the link is available at the given time.
func (l *Link) isUp(time int) bool {
	up := false
	for _, state := range l.states {
		if time >= state.time && state.status == UP {
			up = true
			continue
		}
		if time >= state.time && state.status == DOWN {
			up = false
			continue
		}
	}
	return up
}
