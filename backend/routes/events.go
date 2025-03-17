package routes

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/b-j-roberts/foc-fun/backend/internal/db"
	routeutils "github.com/b-j-roberts/foc-fun/backend/routes/utils"
)

func InitEventsRoutes() {
	http.HandleFunc("/events/get-latest", getLatestEvent)
	http.HandleFunc("/events/get-events", getEvents)
	http.HandleFunc("/events/get-events-from", getEventsFrom)
	http.HandleFunc("/events/get-latest-with", getLatestEventWith)
	http.HandleFunc("/events/get-events-ordered", getEventsOrdered)
	http.HandleFunc("/events/get-events-ordered-data", getEventsOrderedData)
}

type Event struct {
	ID      int      `json:"id"`
	EventId int      `json:"event_id"`
	Keys    []string `json:"keys"`
	Data    []string `json:"data"`
}

func getLatestEvent(w http.ResponseWriter, r *http.Request) {
	eventIdStr := r.URL.Query().Get("eventId")
	if eventIdStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing eventId")
		return
	}
	eventId, err := strconv.Atoi(eventIdStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid eventId")
		return
	}

	query := "SELECT * FROM processedevents WHERE event_id=$1 ORDER BY id DESC LIMIT 1"
	event, err := db.PostgresQueryOneJson[Event](query, eventId)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusInternalServerError, "Error fetching latest event")
		return
	}

	routeutils.WriteDataJson(w, string(event))
}

func getEvents(w http.ResponseWriter, r *http.Request) {
	eventIdStr := r.URL.Query().Get("eventId")
	if eventIdStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing eventId")
		return
	}
	eventId, err := strconv.Atoi(eventIdStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid eventId")
		return
	}

	pageLength, err := strconv.Atoi(r.URL.Query().Get("pageLength"))
	if err != nil || pageLength < 1 {
		pageLength = 10
	}
	if pageLength > 30 {
		pageLength = 30
	}
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	offset := (page - 1) * pageLength

	query := "SELECT * FROM processedevents WHERE event_id=$1 ORDER BY id ASC LIMIT $2 OFFSET $3"
	events, err := db.PostgresQueryJson[Event](query, eventId, pageLength, offset)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusInternalServerError, "Error fetching events")
		return
	}

	routeutils.WriteDataJson(w, string(events))
}

func getEventsFrom(w http.ResponseWriter, r *http.Request) {
	eventIdStr := r.URL.Query().Get("eventId")
	if eventIdStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing eventId")
		return
	}
	eventId, err := strconv.Atoi(eventIdStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid eventId")
		return
	}

	cursor := r.URL.Query().Get("cursor")
	if cursor == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing cursor")
		return
	}
	cursorInt, err := strconv.Atoi(cursor)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid cursor")
		return
	}
	pageLength, err := strconv.Atoi(r.URL.Query().Get("pageLength"))
	if err != nil || pageLength < 1 {
		pageLength = 10
	}
	if pageLength > 30 {
		pageLength = 30
	}

	query := "SELECT * FROM processedevents WHERE event_id=$1 AND id > $2 ORDER BY id ASC LIMIT $3"
	events, err := db.PostgresQueryJson[Event](query, eventId, cursorInt, pageLength)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusInternalServerError, "Error fetching events")
		return
	}

	routeutils.WriteDataJson(w, string(events))
}

type KeysFilter struct {
	Idx int    `json:"idx"`
	Key string `json:"key"`
}

func getLatestEventWith(w http.ResponseWriter, r *http.Request) {
	// Get the latest event for a specific eventId and key
	eventIdStr := r.URL.Query().Get("eventId")
	if eventIdStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing eventId")
		return
	}
	eventId, err := strconv.Atoi(eventIdStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid eventId")
		return
	}

	// Get the keys to filter by
	keysFilterRaw := r.URL.Query()["keys"]
	if len(keysFilterRaw) == 0 {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing keys")
		return
	}
	// Convert keys to a slice of KeysFilter
	// keysFilterRaw is a slice of strings like <keyidx>:<keyvalue>,<keyidx>:<keyvalue>,...
	keysFilter := make([]KeysFilter, len(keysFilterRaw))
	for i, key := range keysFilterRaw {
		keyParts := strings.Split(key, ":")
		if len(keyParts) != 2 {
			routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid key format")
			return
		}
		idx, err := strconv.Atoi(keyParts[0])
		if err != nil {
			routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid key index")
			return
		}
		keysFilter[i] = KeysFilter{Idx: idx, Key: keyParts[1]}
	}

	query := "SELECT * FROM processedevents WHERE event_id=$1 AND ("
	for i := range keysFilter {
		if i > 0 {
			query += " AND "
		}
		query += "keys[$" + strconv.Itoa(i*2+2) + "] = $" + strconv.Itoa(i*2+3)
	}
	query += ") ORDER BY id DESC LIMIT 1"
	// Prepare the args for the query
	args := make([]interface{}, len(keysFilter)*2+1)
	args[0] = eventId
	for i, key := range keysFilter {
		args[i*2+1] = key.Idx
		args[i*2+2] = key.Key
	}
	// Execute the query
	event, err := db.PostgresQueryOneJson[Event](query, args...)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusInternalServerError, "Error fetching latest event")
		return
	}
	if event == nil {
		routeutils.WriteErrorJson(w, http.StatusNotFound, "No event found for the given keys")
		return
	}
	routeutils.WriteDataJson(w, string(event))
}

func getEventsOrdered(w http.ResponseWriter, r *http.Request) {
	// Order events by a specific key
	eventIdStr := r.URL.Query().Get("eventId")
	if eventIdStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing eventId")
		return
	}
	eventId, err := strconv.Atoi(eventIdStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid eventId")
		return
	}

	keyIdxStr := r.URL.Query().Get("keyIdx")
	if keyIdxStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing keyIdx")
		return
	}
	keyIdx, err := strconv.Atoi(keyIdxStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid keyIdx")
		return
	}

	order := r.URL.Query().Get("order")
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	pageLength, err := strconv.Atoi(r.URL.Query().Get("pageLength"))
	if err != nil || pageLength < 1 {
		pageLength = 10
	}
	if pageLength > 30 {
		pageLength = 30
	}
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	offset := (page - 1) * pageLength
	orderBy := "keys[$1] " + order

	query := "SELECT * FROM processedevents WHERE event_id=$2 ORDER BY " + orderBy + " LIMIT $3 OFFSET $4"
	events, err := db.PostgresQueryJson[Event](query, keyIdx, eventId, pageLength, offset)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusInternalServerError, "Error fetching events")
		return
	}
	if events == nil {
		routeutils.WriteErrorJson(w, http.StatusNotFound, "No events found for the given eventId")
		return
	}
	routeutils.WriteDataJson(w, string(events))
}

func getEventsOrderedData(w http.ResponseWriter, r *http.Request) {
	// Order events by a specific data value with a unique key
	eventIdStr := r.URL.Query().Get("eventId")
	if eventIdStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing eventId")
		return
	}
	eventId, err := strconv.Atoi(eventIdStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid eventId")
		return
	}
	dataIdxStr := r.URL.Query().Get("dataIdx")
	if dataIdxStr == "" {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Missing dataIdx")
		return
	}
	dataIdx, err := strconv.Atoi(dataIdxStr)
	if err != nil {
		routeutils.WriteErrorJson(w, http.StatusBadRequest, "Invalid dataIdx")
		return
	}

	order := r.URL.Query().Get("order")
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	pageLength, err := strconv.Atoi(r.URL.Query().Get("pageLength"))
	if err != nil || pageLength < 1 {
		pageLength = 10
	}
	if pageLength > 30 {
		pageLength = 30
	}
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	offset := (page - 1) * pageLength

	uniqueKeyIdx := r.URL.Query().Get("uniqueKey")
	useUniqueKey := false
	if uniqueKeyIdx != "" {
		useUniqueKey = true
	}

	var query string
	if useUniqueKey {
		query = "SELECT DISTINCT ON (keys[$1]) * FROM processedevents WHERE event_id=$2 ORDER BY data[$3] " + order + ", keys[$1] LIMIT $4 OFFSET $5"
		events, err := db.PostgresQueryJson[Event](query, uniqueKeyIdx, eventId, dataIdx, pageLength, offset)
		if err != nil {
			routeutils.WriteErrorJson(w, http.StatusInternalServerError, "Error fetching events")
			return
		}
		if events == nil {
			routeutils.WriteErrorJson(w, http.StatusNotFound, "No events found for the given eventId")
			return
		}
		routeutils.WriteDataJson(w, string(events))
	} else {
		query = "SELECT * FROM processedevents WHERE event_id=$2 ORDER BY data[$3] " + order + " LIMIT $4 OFFSET $5"
		events, err := db.PostgresQueryJson[Event](query, eventId, dataIdx, pageLength, offset)
		if err != nil {
			routeutils.WriteErrorJson(w, http.StatusInternalServerError, "Error fetching events")
			return
		}
		if events == nil {
			routeutils.WriteErrorJson(w, http.StatusNotFound, "No events found for the given eventId")
			return
		}
		routeutils.WriteDataJson(w, string(events))
	}
}
