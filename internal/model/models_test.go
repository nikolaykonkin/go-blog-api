package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUser_ToResponse(t *testing.T) {
	now := time.Now()
	user := User{
		ID:        1,
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "some-bcrypt-hash",
		CreatedAt: now,
		UpdatedAt: now,
	}

	resp := user.ToResponse()

	if resp.ID != user.ID || resp.Username != user.Username || resp.Email != user.Email {
		t.Fatalf("ToResponse did not preserve basic fields: %+v", resp)
	}
	if !resp.CreatedAt.Equal(user.CreatedAt) {
		t.Fatalf("expected CreatedAt to be preserved")
	}

	t.Run("password never appears in the JSON representation of either the model or the response", func(t *testing.T) {
		userJSON, err := json.Marshal(user)
		if err != nil {
			t.Fatalf("unexpected error marshaling user: %v", err)
		}
		if strings.Contains(string(userJSON), "some-bcrypt-hash") {
			t.Fatal("User JSON must never include the password hash (json:\"-\" tag)")
		}

		respJSON, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("unexpected error marshaling response: %v", err)
		}
		if strings.Contains(string(respJSON), "some-bcrypt-hash") {
			t.Fatal("UserResponse JSON must never include the password hash")
		}
	})
}

func TestPost_PermissionChecks(t *testing.T) {
	post := Post{ID: 1, AuthorID: 10}

	if !post.CanBeEditedBy(10) {
		t.Error("expected the author to be able to edit the post")
	}
	if post.CanBeEditedBy(99) {
		t.Error("expected a non-author to not be able to edit the post")
	}

	if !post.CanBeDeletedBy(10) {
		t.Error("expected the author to be able to delete the post")
	}
	if post.CanBeDeletedBy(99) {
		t.Error("expected a non-author to not be able to delete the post")
	}
}

func TestComment_PermissionChecks(t *testing.T) {
	comment := Comment{ID: 1, PostID: 5, AuthorID: 20}

	if !comment.CanBeEditedBy(20) {
		t.Error("expected the author to be able to edit the comment")
	}
	if comment.CanBeEditedBy(21) {
		t.Error("expected a non-author to not be able to edit the comment")
	}

	if !comment.CanBeDeletedBy(20) {
		t.Error("expected the author to be able to delete the comment")
	}
	if comment.CanBeDeletedBy(21) {
		t.Error("expected a non-author to not be able to delete the comment")
	}
}

func TestCommentCreateRequest_PostIDNotSettableFromJSON(t *testing.T) {
	// PostID помечен json:"-", чтобы клиент не мог подменить пост через тело запроса:
	// его выставляет только handler из URL
	body := `{"content":"nice post!","post_id":999}`

	var req CommentCreateRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	if req.PostID != 0 {
		t.Fatalf("expected PostID to be ignored by JSON unmarshaling, got %d", req.PostID)
	}
	if req.Content != "nice post!" {
		t.Fatalf("expected content to be unmarshaled normally, got %q", req.Content)
	}
}
