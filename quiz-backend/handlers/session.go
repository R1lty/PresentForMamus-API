package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"

	"quiz-backend/db"
	"quiz-backend/models"
)

// POST /sessions — Admin creates a new game session and receives the 6-digit join code
func CreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GameID int `json:"game_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GameID <= 0 {
		http.Error(w, "Invalid game_id", http.StatusBadRequest)
		return
	}

	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	var session models.GameSession
	err := db.DB.QueryRow(
		context.Background(),
		`INSERT INTO game_sessions (game_id, code)
		 VALUES ($1, $2)
		 RETURNING id, game_id, code, status, question_status, created_at`,
		req.GameID, code,
	).Scan(&session.ID, &session.GameID, &session.Code, &session.Status, &session.QuestionStatus, &session.CreatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

// GET /sessions/{id} — Get session info: status, current question, who buzzed
func GetSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var session models.GameSession
	err := db.DB.QueryRow(
		context.Background(),
		`SELECT id, game_id, code, status, current_question_id, buzzing_player_id, question_status, created_at
		 FROM game_sessions
		 WHERE id = $1`,
		id,
	).Scan(
		&session.ID, &session.GameID, &session.Code, &session.Status,
		&session.CurrentQuestionID, &session.BuzzingPlayerID,
		&session.QuestionStatus, &session.CreatedAt,
	)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// POST /sessions/join — Player joins a session by entering the 6-digit code and their token
func JoinSession(w http.ResponseWriter, r *http.Request) {
	var req models.JoinSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var playerID int
	err := db.DB.QueryRow(
		context.Background(),
		`SELECT id FROM players WHERE token = $1 AND is_blocked = false`,
		req.Token,
	).Scan(&playerID)
	if err != nil {
		http.Error(w, "Player not found or blocked", http.StatusNotFound)
		return
	}

	var sessionID int
	var sessionStatus string
	err = db.DB.QueryRow(
		context.Background(),
		`SELECT id, status FROM game_sessions WHERE code = $1`,
		req.Code,
	).Scan(&sessionID, &sessionStatus)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if sessionStatus != "waiting" {
		http.Error(w, "Session is no longer accepting players", http.StatusConflict)
		return
	}

	var sp models.SessionPlayer
	err = db.DB.QueryRow(
		context.Background(),
		`INSERT INTO session_players (session_id, player_id)
		 VALUES ($1, $2)
		 ON CONFLICT (session_id, player_id) DO UPDATE SET joined_at = session_players.joined_at
		 RETURNING id, session_id, player_id, score, joined_at`,
		sessionID, playerID,
	).Scan(&sp.ID, &sp.SessionID, &sp.PlayerID, &sp.Score, &sp.JoinedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sp)
}

// POST /sessions/{id}/start — Admin starts the game (moves status from 'waiting' to 'active')
func StartSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	tag, err := db.DB.Exec(
		context.Background(),
		`UPDATE game_sessions SET status = 'active' WHERE id = $1 AND status = 'waiting'`,
		id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "Session not found or already started", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /sessions/{id}/finish — Admin ends the game session
func FinishSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	tag, err := db.DB.Exec(
		context.Background(),
		`UPDATE game_sessions
		 SET status = 'finished', question_status = 'idle', buzzing_player_id = NULL, current_question_id = NULL
		 WHERE id = $1 AND status = 'active'`,
		id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "Session not found or not active", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /sessions/{id}/players — Get all players currently in a session with their scores
func GetSessionPlayers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	rows, err := db.DB.Query(
		context.Background(),
		`SELECT p.id, p.name, sp.score, sp.joined_at
		 FROM session_players sp
		 JOIN players p ON p.id = sp.player_id
		 WHERE sp.session_id = $1
		 ORDER BY sp.joined_at ASC`,
		id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	players := []models.SessionPlayerInfo{}
	for rows.Next() {
		var p models.SessionPlayerInfo
		if err := rows.Scan(&p.PlayerID, &p.PlayerName, &p.Score, &p.JoinedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		players = append(players, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(players)
}

// GET /sessions/{id}/leaderboard — Get session leaderboard sorted by score descending
func GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	rows, err := db.DB.Query(
		context.Background(),
		`SELECT p.id, p.name, sp.score
		 FROM session_players sp
		 JOIN players p ON p.id = sp.player_id
		 WHERE sp.session_id = $1
		 ORDER BY sp.score DESC`,
		id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	leaderboard := []models.LeaderboardEntry{}
	for rows.Next() {
		var entry models.LeaderboardEntry
		if err := rows.Scan(&entry.PlayerID, &entry.PlayerName, &entry.Score); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		leaderboard = append(leaderboard, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}

// POST /sessions/{id}/question — Admin picks a question; players can now buzz in
func SetActiveQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req models.SetQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.QuestionID <= 0 {
		http.Error(w, "Invalid question_id", http.StatusBadRequest)
		return
	}

	tag, err := db.DB.Exec(
		context.Background(),
		`UPDATE game_sessions
		 SET current_question_id = $1, question_status = 'open', buzzing_player_id = NULL
		 WHERE id = $2 AND status = 'active'`,
		req.QuestionID, id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "Session not found or not active", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /sessions/{id}/buzz — Player hits the buzz button; first one to send this wins the right to answer
func BuzzIn(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req models.BuzzRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var playerID int
	err := db.DB.QueryRow(
		context.Background(),
		`SELECT id FROM players WHERE token = $1 AND is_blocked = false`,
		req.Token,
	).Scan(&playerID)
	if err != nil {
		http.Error(w, "Player not found or blocked", http.StatusUnauthorized)
		return
	}

	// Atomic update: only succeeds if nobody has buzzed yet AND the player is in this session
	tag, err := db.DB.Exec(
		context.Background(),
		`UPDATE game_sessions
		SET buzzing_player_id = $1, question_status = 'buzzing'
		WHERE id = $2
		AND status = 'active'
		AND question_status = 'open'
		AND current_question_id IS NOT NULL
		AND EXISTS (
			SELECT 1 FROM session_players
			WHERE session_id = $2 AND player_id = $1
		)
		AND NOT EXISTS (
			SELECT 1
			FROM session_question_wrong_answers wa
			WHERE wa.session_id = game_sessions.id
				AND wa.question_id = game_sessions.current_question_id
				AND wa.player_id = $1
		)`,
		playerID, id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "Cannot buzz: question is closed, someone already buzzed, or you are not in this session", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /sessions/{id}/judge — Admin decides if the buzzing player's answer is correct or not
func JudgeAnswer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req models.JudgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var currentQuestionID, buzzingPlayerID int
	err := db.DB.QueryRow(
		context.Background(),
		`SELECT current_question_id, buzzing_player_id
		 FROM game_sessions
		 WHERE id = $1 AND status = 'active' AND question_status = 'buzzing'`,
		id,
	).Scan(&currentQuestionID, &buzzingPlayerID)
	if err != nil {
		http.Error(w, "Session not found or not in buzzing state", http.StatusConflict)
		return
	}

	var points int
	err = db.DB.QueryRow(
		context.Background(),
		`SELECT points_per_question FROM questions WHERE id = $1`,
		currentQuestionID,
	).Scan(&points)
	if err != nil {
		http.Error(w, "Question not found", http.StatusInternalServerError)
		return
	}

	if req.Correct {
		_, err = db.DB.Exec(
			context.Background(),
			`UPDATE session_players
			 SET score = score + $1
			 WHERE session_id = $2 AND player_id = $3`,
			points, id, buzzingPlayerID,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = db.DB.Exec(
			context.Background(),
			`UPDATE game_sessions
			 SET question_status = 'idle',
			     buzzing_player_id = NULL,
			     current_question_id = NULL
			 WHERE id = $1`,
			id,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	_, err = db.DB.Exec(
		context.Background(),
		`UPDATE session_players
		 SET score = score - $1
		 WHERE session_id = $2 AND player_id = $3`,
		points, id, buzzingPlayerID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = db.DB.Exec(
		context.Background(),
		`INSERT INTO session_question_wrong_answers (session_id, question_id, player_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (session_id, question_id, player_id) DO NOTHING`,
		id, currentQuestionID, buzzingPlayerID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var playersCount int
	err = db.DB.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
		 FROM session_players
		 WHERE session_id = $1`,
		id,
	).Scan(&playersCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wrongAnswersCount int
	err = db.DB.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
		 FROM session_question_wrong_answers
		 WHERE session_id = $1 AND question_id = $2`,
		id, currentQuestionID,
	).Scan(&wrongAnswersCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if wrongAnswersCount >= playersCount {
		_, err = db.DB.Exec(
			context.Background(),
			`UPDATE game_sessions
			 SET question_status = 'idle',
			     buzzing_player_id = NULL,
			     current_question_id = NULL
			 WHERE id = $1`,
			id,
		)
	} else {
		_, err = db.DB.Exec(
			context.Background(),
			`UPDATE game_sessions
			 SET question_status = 'open',
			     buzzing_player_id = NULL
			 WHERE id = $1`,
			id,
		)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /sessions/{id}/player-state?token=... — Get player state in current session
func GetPlayerSessionState(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	var playerID int
	err := db.DB.QueryRow(
		context.Background(),
		`SELECT id FROM players WHERE token = $1 AND is_blocked = false`,
		token,
	).Scan(&playerID)
	if err != nil {
		http.Error(w, "Player not found or blocked", http.StatusUnauthorized)
		return
	}

	var score int
	err = db.DB.QueryRow(
		context.Background(),
		`SELECT score
		 FROM session_players
		 WHERE session_id = $1 AND player_id = $2`,
		id, playerID,
	).Scan(&score)
	if err != nil {
		http.Error(w, "Player is not in this session", http.StatusNotFound)
		return
	}

	var sessionStatus string
	var questionStatus string
	var currentQuestionID sql.NullInt64

	err = db.DB.QueryRow(
		context.Background(),
		`SELECT status, question_status, current_question_id
		 FROM game_sessions
		 WHERE id = $1`,
		id,
	).Scan(&sessionStatus, &questionStatus, &currentQuestionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	isBlockedForQuestion := false

	if currentQuestionID.Valid {
		var wrongCount int
		err = db.DB.QueryRow(
			context.Background(),
			`SELECT COUNT(*)
			 FROM session_question_wrong_answers
			 WHERE session_id = $1
			   AND question_id = $2
			   AND player_id = $3`,
			id, int(currentQuestionID.Int64), playerID,
		).Scan(&wrongCount)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		isBlockedForQuestion = wrongCount > 0
	}

	canBuzz := sessionStatus == "active" &&
		questionStatus == "open" &&
		currentQuestionID.Valid &&
		!isBlockedForQuestion

	state := models.PlayerSessionState{
		PlayerID:             playerID,
		Score:                score,
		IsBlockedForQuestion: isBlockedForQuestion,
		CanBuzz:              canBuzz,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// GET /session-by-code/{code} — Get session by room code
func GetSessionByCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	if code == "" {
		http.Error(w, "Missing code", http.StatusBadRequest)
		return
	}

	var session models.GameSession

	err := db.DB.QueryRow(
		context.Background(),
		`SELECT id, game_id, code, status, current_question_id, buzzing_player_id, question_status, created_at
		 FROM game_sessions
		 WHERE code = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		code,
	).Scan(
		&session.ID,
		&session.GameID,
		&session.Code,
		&session.Status,
		&session.CurrentQuestionID,
		&session.BuzzingPlayerID,
		&session.QuestionStatus,
		&session.CreatedAt,
	)

	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}
