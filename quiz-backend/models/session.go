package models

import "time"

type GameSession struct {
	ID                int       `json:"id"`
	GameID            int       `json:"game_id"`
	Code              string    `json:"code"`
	Status            string    `json:"status"`
	CurrentQuestionID *int      `json:"current_question_id,omitempty"`
	BuzzingPlayerID   *int      `json:"buzzing_player_id,omitempty"`
	QuestionStatus    string    `json:"question_status"`
	CreatedAt         time.Time `json:"created_at"`
}

type SessionPlayer struct {
	ID        int       `json:"id"`
	SessionID int       `json:"session_id"`
	PlayerID  int       `json:"player_id"`
	Score     int       `json:"score"`
	JoinedAt  time.Time `json:"joined_at"`
}

type SessionPlayerInfo struct {
	PlayerID   int       `json:"player_id"`
	PlayerName string    `json:"player_name"`
	Score      int       `json:"score"`
	JoinedAt   time.Time `json:"joined_at"`
}

type JoinSessionRequest struct {
	Code  string `json:"code"`
	Token string `json:"token"`
}

type SetQuestionRequest struct {
	QuestionID int `json:"question_id"`
}

type BuzzRequest struct {
	Token string `json:"token"`
}

type JudgeRequest struct {
	Correct bool `json:"correct"`
}

type LeaderboardEntry struct {
	PlayerID   int    `json:"player_id"`
	PlayerName string `json:"player_name"`
	Score      int    `json:"score"`
}

type PlayerSessionState struct {
	PlayerID             int  `json:"player_id"`
	Score                int  `json:"score"`
	IsBlockedForQuestion bool `json:"is_blocked_for_question"`
	CanBuzz              bool `json:"can_buzz"`
}
