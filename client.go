package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	// "fmt"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// connection is an middleman between the websocket connection and the hub.
type connection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// SocketEvent is the event structure used for socket messages
type SocketEvent struct {
	EventType    string `json:"type"`
	EventValue   string `json:"value"`
	EventWarrior string `json:"warriorId"`
}

// CreateSocketEvent makes a SocketEvent struct and turns it into json []byte
func CreateSocketEvent(EventType string, EventValue string, EventWarrior string) []byte {
	newEvent := &SocketEvent{
		EventType:    EventType,
		EventValue:   EventValue,
		EventWarrior: EventWarrior,
	}

	event, _ := json.Marshal(newEvent)

	return event
}

// readPump pumps messages from the websocket connection to the hub.
func (s subscription) readPump() {
	c := s.conn
	defer func() {
		BattleID := s.arena
		WarriorID := s.warriorID

		Warriors := RetreatWarrior(BattleID, WarriorID)
		updatedWarriors, _ := json.Marshal(Warriors)

		retreatEvent := CreateSocketEvent("warrior_retreated", string(updatedWarriors), WarriorID)
		m := message{retreatEvent, BattleID}
		h.broadcast <- m

		h.unregister <- s
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}

		var badEvent bool
		keyVal := make(map[string]string)
		json.Unmarshal(msg, &keyVal) // check for errors
		warriorID := s.warriorID
		battleID := s.arena

		switch keyVal["type"] {
		case "vote":
			voteObj := make(map[string]string)
			json.Unmarshal([]byte(keyVal["value"]), &voteObj)
			VoteValue := voteObj["voteValue"]
			PlanID := voteObj["planId"]

			plans := SetVote(battleID, warriorID, PlanID, VoteValue)

			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("vote_activity", string(updatedPlans), warriorID)

			if areAllWarriorsVoted(PlanID, battleID) {
				log.Println("All warrioirs voted")

				plans, err :=  EndPlanVoting(battleID, warriorID, PlanID)

				if err != nil {
					log.Println(err)
				}

				updatedPlans, _ := json.Marshal(plans)

				msg = CreateSocketEvent("voting_ended", string(updatedPlans), "")
			}
		case "retract_vote":
			PlanID := keyVal["value"]

			plans := RetractVote(battleID, warriorID, PlanID)

			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("vote_retracted", string(updatedPlans), warriorID)
		case "add_plan":
			plans, err := CreatePlan(battleID, warriorID, keyVal["value"])
			if err != nil {
				badEvent = true
				break
			}
			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("plan_added", string(updatedPlans), "")
		case "activate_plan":
			plans, err := ActivatePlanVoting(battleID, warriorID, keyVal["value"])
			if err != nil {
				badEvent = true
				break
			}
			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("plan_activated", string(updatedPlans), "")
		case "skip_plan":
			plans, err := SkipPlan(battleID, warriorID, keyVal["value"])
			if err != nil {
				badEvent = true
				break
			}
			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("plan_skipped", string(updatedPlans), "")
		case "end_voting":
			plans, err := EndPlanVoting(battleID, warriorID, keyVal["value"])
			if err != nil {
				badEvent = true
				break
			}
			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("voting_ended", string(updatedPlans), "")
		case "finalize_plan":
			planObj := make(map[string]string)
			json.Unmarshal([]byte(keyVal["value"]), &planObj)
			PlanID := planObj["planId"]
			PlanPoints := planObj["planPoints"]

			plans, err := FinalizePlan(battleID, warriorID, PlanID, PlanPoints)
			if err != nil {
				badEvent = true
				break
			}
			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("plan_finalized", string(updatedPlans), "")
		case "revise_plan":
			planObj := make(map[string]string)
			json.Unmarshal([]byte(keyVal["value"]), &planObj)
			PlanID := planObj["planId"]
			PlanName := planObj["planName"]

			plans, err := RevisePlanName(battleID, warriorID, PlanID, PlanName)
			if err != nil {
				badEvent = true
				break
			}
			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("plan_revised", string(updatedPlans), "")
		case "burn_plan":
			plans, err := BurnPlan(battleID, warriorID, keyVal["value"])
			if err != nil {
				badEvent = true
				break
			}
			updatedPlans, _ := json.Marshal(plans)
			msg = CreateSocketEvent("plan_burned", string(updatedPlans), "")
		case "promote_leader":
			battle, err := SetBattleLeader(battleID, warriorID, keyVal["value"])
			if err != nil {
				badEvent = true
				break
			}

			updatedBattle, _ := json.Marshal(battle)
			msg = CreateSocketEvent("battle_updated", string(updatedBattle), "")
		case "concede_battle":
			err := DeleteBattle(battleID, warriorID)
			if err != nil {
				badEvent = true
				break
			}
			msg = CreateSocketEvent("battle_conceded", "", "")
		case "jab_warrior":
			err := ConfirmLeader(battleID, warriorID)
			if err != nil {
				badEvent = true
				break
			}
		default:
		}

		if badEvent != true {
			m := message{msg, s.arena}
			h.broadcast <- m
		}
	}
}

// write writes a message with the given message type and payload.
func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (s *subscription) writePump() {
	c := s.conn
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// ClearWarriorCookies wipes the frontend and backend cookies
// used in the event of bad cookie reads
func ClearWarriorCookies(w http.ResponseWriter) {
	feCookie := &http.Cookie{
		Name:    "warrior",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	}
	beCookie := &http.Cookie{
		Name:     SecureCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	}

	http.SetCookie(w, feCookie)
	http.SetCookie(w, beCookie)
}

// ValidateWarriorCookie returns the warriorID from secure cookies or errors if failures getting it
func ValidateWarriorCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	var warriorID string

	if cookie, err := r.Cookie(SecureCookieName); err == nil {
		var value string
		if err = Sc.Decode(SecureCookieName, cookie.Value, &value); err == nil {
			warriorID = value
		} else {
			log.Println("error in reading warrior cookie : " + err.Error() + "\n")
			ClearWarriorCookies(w)
			return "", errors.New("invalid warrior cookies")
		}
	} else {
		log.Println("error in reading warrior cookie : " + err.Error() + "\n")
		ClearWarriorCookies(w)
		return "", errors.New("invalid warrior cookies")
	}

	return warriorID, nil
}

// serveWs handles websocket requests from the peer.
func serveWs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	battleID := vars["id"]

	// make sure battle is legit
	b, battleErr := GetBattle(battleID)
	if battleErr != nil {
		http.NotFound(w, r)
		return
	}
	battle, _ := json.Marshal(b)

	// make sure warrior cookies are valid
	warriorID, cookieErr := ValidateWarriorCookie(w, r)
	if cookieErr != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// make sure warrior exists
	_, warErr := GetBattleWarrior(battleID, warriorID)

	if warErr != nil {
		log.Println("error finding warrior : " + warErr.Error() + "\n")
		ClearWarriorCookies(w)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// upgrade to WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	c := &connection{send: make(chan []byte, 256), ws: ws}
	s := subscription{c, battleID, warriorID}
	h.register <- s

	Warriors, _ := AddWarriorToBattle(s.arena, warriorID)
	updatedWarriors, _ := json.Marshal(Warriors)

	initEvent := CreateSocketEvent("init", string(battle), warriorID)
	_ = c.write(websocket.TextMessage, initEvent)

	joinedEvent := CreateSocketEvent("warrior_joined", string(updatedWarriors), warriorID)
	m := message{joinedEvent, s.arena}
	h.broadcast <- m

	go s.writePump()
	s.readPump()
}
