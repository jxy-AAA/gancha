package models

import "time"

type User struct {
	ID             int64      `json:"id"`
	Username       string     `json:"username"`
	Email          string     `json:"email,omitempty"`
	Role           string     `json:"role"`
	Avatar         string     `json:"avatar"`
	Bio            string     `json:"bio"`
	Expertise      string     `json:"expertise"`
	SuspendedUntil *time.Time `json:"suspended_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Category struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
	Position int    `json:"position"`
}

type Question struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	CategoryID       int64     `json:"category_id"`
	CategoryName     string    `json:"category_name"`
	Title            string    `json:"title"`
	Body             string    `json:"body"`
	Tags             string    `json:"tags"`
	Views            int       `json:"views"`
	Status           string    `json:"status"`
	AcceptedAnswerID int64     `json:"accepted_answer_id"`
	Score            int       `json:"score"`
	AnswerCount      int       `json:"answer_count"`
	Author           string    `json:"author"`
	AuthorAvatar     string    `json:"author_avatar"`
	CreatedAt        time.Time `json:"created_at"`
	EditedAt         *time.Time `json:"edited_at,omitempty"`
}

type Answer struct {
	ID         int64      `json:"id"`
	QuestionID int64      `json:"question_id"`
	UserID     int64      `json:"user_id"`
	Body       string     `json:"body"`
	Author     string     `json:"author"`
	AuthorRole string     `json:"author_role"`
	Avatar     string     `json:"avatar"`
	Score      int        `json:"score"`
	Accepted   bool       `json:"accepted"`
	CreatedAt  time.Time  `json:"created_at"`
	EditedAt   *time.Time `json:"edited_at,omitempty"`
}

type Article struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Title      string     `json:"title"`
	Summary    string     `json:"summary"`
	Body       string     `json:"body"`
	Tags       string     `json:"tags"`
	Views      int        `json:"views"`
	Published  bool       `json:"published"`
	Author     string     `json:"author"`
	AuthorRole string     `json:"author_role"`
	Avatar     string     `json:"avatar"`
	CreatedAt  time.Time  `json:"created_at"`
	EditedAt   *time.Time `json:"edited_at,omitempty"`
}

type Board struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Position    int    `json:"position"`
	PostCount   int    `json:"post_count"`
}

type ForumPost struct {
	ID         int64      `json:"id"`
	BoardID    int64      `json:"board_id"`
	BoardName  string     `json:"board_name"`
	UserID     int64      `json:"user_id"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	Views      int        `json:"views"`
	Author     string     `json:"author"`
	Avatar     string     `json:"avatar"`
	ReplyCount int        `json:"reply_count"`
	CreatedAt  time.Time  `json:"created_at"`
	EditedAt   *time.Time `json:"edited_at,omitempty"`
}

type ForumReply struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	UserID    int64     `json:"user_id"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
}

type Comment struct {
	ID        int64     `json:"id"`
	QuestionID int64    `json:"question_id"`
	UserID    int64     `json:"user_id"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
}

type Notification struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Type       string    `json:"type"`
	Message    string    `json:"message"`
	QuestionID *int64    `json:"question_id,omitempty"`
	Read       bool      `json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}

type AuditEntry struct {
	ID         int64     `json:"id"`
	ActorID    int64     `json:"actor_id"`
	ActorName  string    `json:"actor_name"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   *int64    `json:"target_id,omitempty"`
	Details    string    `json:"details"`
	CreatedAt  time.Time `json:"created_at"`
}

type Stats struct {
	Users        int `json:"users"`
	Questions    int `json:"questions"`
	Answers      int `json:"answers"`
	Articles     int `json:"articles"`
	ForumPosts   int `json:"forum_posts"`
	Uploads      int `json:"uploads"`
	Notifications int `json:"notifications"`
}
