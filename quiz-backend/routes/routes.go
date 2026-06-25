package routes

import (
	"net/http"

	"quiz-backend/handlers"
)

func SetupRoutes() *http.ServeMux {
	router := http.NewServeMux()

	// Игроки
	router.HandleFunc("POST /create-player", handlers.CreatePlayer)

	// Категории
	router.HandleFunc("/categories", handlers.CategoriesHandler)
	router.HandleFunc("/categories/{id}", handlers.CategoryByIDHandler)
	router.HandleFunc("/categories/{id}/round", handlers.CategoryRoundHandler)

	// Вопросы
	router.HandleFunc("/questions", handlers.QuestionsHandler)
	router.HandleFunc("/questions/{id}", handlers.QuestionByIDHandler)

	// Игры
	router.HandleFunc("/games", handlers.GameHandler)
	router.HandleFunc("/games/{id}", handlers.GameByIDHandler)
	router.HandleFunc("/games/{game_id}/rounds/{round_id}/categories", handlers.CategoriesByRoundHandler)

	// Раунды
	router.HandleFunc("/rounds", handlers.RoundHandler)
	router.HandleFunc("/rounds/{id}", handlers.RoundByIDHandler)

	// Игровые сессии — лобби и ход игры
	router.HandleFunc("POST /sessions", handlers.CreateSession)                   // Админ создаёт сессию, получает 6-значный код
	router.HandleFunc("GET /sessions/{id}", handlers.GetSession)                  // Получить состояние сессии (статус, текущий вопрос, кто нажал кнопку)
	router.HandleFunc("POST /sessions/join", handlers.JoinSession)                // Игрок заходит в сессию по коду и токену
	router.HandleFunc("GET /sessions/{id}/players", handlers.GetSessionPlayers)   // Список игроков в сессии с очками
	router.HandleFunc("POST /sessions/{id}/start", handlers.StartSession)         // Админ запускает игру
	router.HandleFunc("POST /sessions/{id}/finish", handlers.FinishSession)       // Админ завершает игру
	router.HandleFunc("POST /sessions/{id}/question", handlers.SetActiveQuestion) // Админ выбирает вопрос, открывает нажатие кнопки
	router.HandleFunc("POST /sessions/{id}/buzz", handlers.BuzzIn)                // Игрок нажимает кнопку (первый выигрывает право ответа)
	router.HandleFunc("POST /sessions/{id}/judge", handlers.JudgeAnswer)          // Админ засчитывает или не засчитывает ответ
	router.HandleFunc("GET /sessions/{id}/leaderboard", handlers.GetLeaderboard)  // Таблица лидеров по очкам
	router.HandleFunc("GET /sessions/{id}/player-state", handlers.GetPlayerSessionState)
	router.HandleFunc("GET /session-by-code/{code}", handlers.GetSessionByCode)
	router.HandleFunc("GET /ws/sessions/{id}", handlers.SessionWebSocket)
	return router
}
