package model

import (
	"time"
)

type User struct {
	ID        int       `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password"` // хеш пароля, в JSON не попадает
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Post struct {
	ID        int       `json:"id" db:"id"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	AuthorID  int       `json:"author_id" db:"author_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Comment — комментарий к посту
// UpdatedAt нужен CommentRepo.Update: тот же запрос обновляет колонку updated_at
type Comment struct {
	ID        int       `json:"id" db:"id"`
	Content   string    `json:"content" db:"content"`
	PostID    int       `json:"post_id" db:"post_id"`
	AuthorID  int       `json:"author_id" db:"author_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type UserCreateRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type UserLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type PostCreateRequest struct {
	Title   string `json:"title" validate:"required,min=1,max=200"`
	Content string `json:"content" validate:"required,min=1"`
}

type PostUpdateRequest struct {
	Title   string `json:"title" validate:"required,min=1,max=200"`
	Content string `json:"content" validate:"required,min=1"`
}

// CommentCreateRequest получает PostID от handler из URL маршрута, а не из тела запроса
// Тег json:"-" не дает клиенту подменить его, а handler всегда перезаписывает поле
type CommentCreateRequest struct {
	Content string `json:"content" validate:"required,min=1,max=1000"`
	PostID  int    `json:"-"`
}

type CommentUpdateRequest struct {
	Content string `json:"content" validate:"required,min=1,max=1000"`
}

// UserResponse — данные пользователя без пароля, но с email
// Отдается только самому пользователю: профиль, регистрация, вход
type UserResponse struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthorResponse — публичные данные автора в постах и комментариях
// Без email: эти ответы доступны анонимным клиентам
type AuthorResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type TokenResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

type PostResponse struct {
	ID        int            `json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Author    AuthorResponse `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type CommentResponse struct {
	ID        int            `json:"id"`
	Content   string         `json:"content"`
	PostID    int            `json:"post_id"`
	Author    AuthorResponse `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

// ToAuthorResponse отдает только публичные поля — без email
func (u *User) ToAuthorResponse() AuthorResponse {
	return AuthorResponse{
		ID:       u.ID,
		Username: u.Username,
	}
}

func (p *Post) CanBeEditedBy(userID int) bool {
	return p.AuthorID == userID
}

func (p *Post) CanBeDeletedBy(userID int) bool {
	return p.AuthorID == userID
}

func (c *Comment) CanBeEditedBy(userID int) bool {
	return c.AuthorID == userID
}

func (c *Comment) CanBeDeletedBy(userID int) bool {
	return c.AuthorID == userID
}
