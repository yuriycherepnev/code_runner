package main

import (
	"code_runner/config"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
)

var (
	postgresDb *sql.DB
)

func main() {
	connStr := config.GetDbUrl()
	var err error
	postgresDb, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer postgresDb.Close()

	err = postgresDb.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	createLangTableSQL := `
	CREATE TABLE IF NOT EXISTS lang (
		id SERIAL PRIMARY KEY,
		name VARCHAR(50) NOT NULL UNIQUE
	);`

	_, err = postgresDb.Exec(createLangTableSQL)
	if err != nil {
		log.Fatalf("Failed to create lang table: %v", err)
	}

	_, err = postgresDb.Exec(`
		INSERT INTO lang (name) 
		VALUES ('Python') 
		ON CONFLICT (name) DO NOTHING`)
	if err != nil {
		log.Fatalf("Failed to insert Python language: %v", err)
	}

	var pythonID int
	err = postgresDb.QueryRow("SELECT id FROM lang WHERE name = 'Python'").Scan(&pythonID)
	if err != nil {
		log.Fatalf("Failed to get Python language ID: %v", err)
	}

	createTaskTableSQL := `
	CREATE TABLE IF NOT EXISTS task (
		id SERIAL PRIMARY KEY,
		text TEXT NOT NULL,
		id_lang INTEGER REFERENCES lang(id) ON DELETE SET NULL
	);
	
	CREATE INDEX IF NOT EXISTS id_task_id_lang ON task(id_lang);
	`

	_, err = postgresDb.Exec(createTaskTableSQL)
	if err != nil {
		log.Fatalf("Failed to create tasks table: %v", err)
	}

	createSolutionTableSQL := `
	CREATE TABLE IF NOT EXISTS solution (
		id SERIAL PRIMARY KEY,
		id_task INTEGER NOT NULL REFERENCES task(id) ON DELETE CASCADE,
		id_user INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
		code TEXT NOT NULL,
		UNIQUE(id_task, id_user)
	);

	CREATE INDEX IF NOT EXISTS id_solution_id_task ON solution(id_task);
	CREATE INDEX IF NOT EXISTS id_solution_id_user ON solution(id_user);
	`

	_, err = postgresDb.Exec(createSolutionTableSQL)
	if err != nil {
		log.Fatalf("Failed to create solutions table: %v", err)
	}

	tasks := []string{
		"найдите сумму чисел от 1 до 10",
		"напишите проверку на палиндром",
		"напишите генератор чисел Фибоначчи до 10 числа",
		"подсчитайте количество гласных в строке",
		"напишите код который переносит строку в массив побуквенно",
	}

	for _, task := range tasks {
		_, err = postgresDb.Exec(`
			INSERT INTO task (text, id_lang) 
			VALUES ($1, $2)`, task, pythonID)
		if err != nil {
			log.Printf("Failed to insert task '%s': %v", task, err)
			continue
		}
	}

	fmt.Println("Migration completed successfully")
	fmt.Println("- Created lang, task and solution tables")
	fmt.Println("- Added Python language")
	fmt.Println("- Added 5 Python tasks")
	fmt.Println("- Created solution table with foreign keys to task and user")
}
