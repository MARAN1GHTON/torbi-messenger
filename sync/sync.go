package sync

import (
	"torbi/database"
)

// SyncMessage represents a serialized message transmitted during synchronization.
type SyncMessage struct {
	ID            string `json:"id"`
	ChatID        string `json:"chat_id"`
	SenderID      string `json:"sender_id"`
	EncryptedBody []byte `json:"encrypted_body"`
	Timestamp     int64  `json:"timestamp"`
	LamportClock  int64  `json:"lamport_clock"`
}

// SyncPayload represents the network envelope for the synchronization protocol.
type SyncPayload struct {
	// Request contains the sender's message IDs: ChatID -> List of Message IDs
	Request map[string][]string `json:"request,omitempty"`

	// Response contains delta messages for the recipient
	Response []SyncMessage `json:"response,omitempty"`
}

// BuildInitialRequest creates a SyncPayload containing the message ID lists for all local chats.
func BuildInitialRequest(db *database.DB) (*SyncPayload, error) {
	chats, err := db.ListChats()
	if err != nil {
		return nil, err
	}

	request := make(map[string][]string)
	for _, c := range chats {
		ids, err := db.GetMessageIDs(c.ID)
		if err != nil {
			return nil, err
		}
		if ids == nil {
			ids = []string{}
		}
		request[c.ID] = ids
	}

	return &SyncPayload{
		Request: request,
	}, nil
}

// ProcessRequestAndResponse processes an incoming SyncPayload.
// It inserts any received delta messages into the local database.
// If the payload contains a request vector, it queries the local database for missing messages
// and returns a response payload. If isStep2 is true, it also includes the local request vector.
func ProcessRequestAndResponse(db *database.DB, payload *SyncPayload, isStep2 bool) (*SyncPayload, error) {
	// 1. Ingest delta messages
	for _, sm := range payload.Response {
		exists, err := db.GetMessage(sm.ID)
		if err != nil {
			return nil, err
		}
		if exists == nil {
			// Message is missing, save it locally
			m := &database.Message{
				ID:            sm.ID,
				ChatID:        sm.ChatID,
				SenderID:      sm.SenderID,
				EncryptedBody: sm.EncryptedBody,
				Timestamp:     sm.Timestamp,
				LamportClock:  sm.LamportClock,
			}
			if err := db.SaveMessage(m); err != nil {
				return nil, err
			}
		}
	}

	// 2. If the payload contains a Request, calculate and return the deltas we have
	if payload.Request != nil {
		var responseMsgs []SyncMessage

		for chatID, remoteIDs := range payload.Request {
			// Verify if we have this chat
			chat, err := db.GetChat(chatID)
			if err != nil {
				return nil, err
			}
			if chat == nil {
				continue // We don't have this chat, nothing to send
			}

			// Get messages except what they already have
			deltas, err := db.GetMessagesExcept(chatID, remoteIDs)
			if err != nil {
				return nil, err
			}

			for _, m := range deltas {
				responseMsgs = append(responseMsgs, SyncMessage{
					ID:            m.ID,
					ChatID:        m.ChatID,
					SenderID:      m.SenderID,
					EncryptedBody: m.EncryptedBody,
					Timestamp:     m.Timestamp,
					LamportClock:  m.LamportClock,
				})
			}
		}

		respPayload := &SyncPayload{
			Response: responseMsgs,
		}

		// If this is Step 2, we also include our own Request vector so they can send us their deltas
		if isStep2 {
			chats, err := db.ListChats()
			if err != nil {
				return nil, err
			}
			localRequest := make(map[string][]string)
			for _, c := range chats {
				ids, err := db.GetMessageIDs(c.ID)
				if err != nil {
					return nil, err
				}
				if ids == nil {
					ids = []string{}
				}
				localRequest[c.ID] = ids
			}
			respPayload.Request = localRequest
		}

		return respPayload, nil
	}

	return nil, nil
}
