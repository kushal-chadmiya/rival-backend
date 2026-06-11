package httpapi

import (
	"fmt"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"taskboard-backend/internal/activity"
	"taskboard-backend/internal/auth"
	"taskboard-backend/internal/tasks"
)

func listAttachmentsHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		taskID := chi.URLParam(r, "taskID")
		if err := tasks.ValidateTaskID(taskID); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		attachments, err := deps.TaskStore.ListAttachments(r.Context(), taskID, viewer.UserID, viewer.IsAdmin())
		if err != nil {
			log.Printf("list attachments: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to load attachments")
			return
		}

		if deps.FileStorage != nil {
			for i := range attachments {
				url, err := deps.FileStorage.SignedURL(r.Context(), attachments[i].StoragePath, time.Hour)
				if err == nil {
					attachments[i].DownloadURL = url
				}
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": attachments})
	}
}

func uploadAttachmentHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}
		if deps.FileStorage == nil {
			writeError(w, http.StatusServiceUnavailable, "storage_unavailable", "file storage is not configured")
			return
		}

		taskID := chi.URLParam(r, "taskID")
		if err := tasks.ValidateTaskID(taskID); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		task, err := deps.TaskStore.GetTask(r.Context(), taskID, viewer.UserID, viewer.IsAdmin())
		if err != nil {
			writeTaskStoreError(w, err, "failed to load task", "task not found")
			return
		}
		if !writeUnlessTaskOwner(w, task.UserID, viewer.UserID) {
			return
		}

		if err := r.ParseMultipartForm(deps.MaxUploadBytes); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "invalid multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "file is required")
			return
		}
		defer file.Close()

		mimeType := header.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		if err := tasks.ValidateAttachment(header.Filename, mimeType, header.Size, deps.MaxUploadBytes); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		storagePath := fmt.Sprintf("%s/%s/%s_%s", viewer.UserID, taskID, uuid.NewString(), path.Base(header.Filename))
		if err := deps.FileStorage.Upload(r.Context(), storagePath, file, mimeType, header.Size); err != nil {
			log.Printf("upload attachment: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to upload file")
			return
		}

		attachment, err := deps.TaskStore.AddAttachment(r.Context(), tasks.CreateAttachmentParams{
			TaskID:      taskID,
			UserID:      viewer.UserID,
			FileName:    path.Base(header.Filename),
			MimeType:    mimeType,
			SizeBytes:   header.Size,
			StoragePath: storagePath,
		})
		if err != nil {
			_ = deps.FileStorage.Delete(r.Context(), storagePath)
			log.Printf("save attachment: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to save attachment")
			return
		}

		if deps.ActivityStore != nil {
			_ = deps.ActivityStore.Record(r.Context(), taskID, viewer, activity.ActionAttachmentAdded, map[string]any{
				"file_name": attachment.FileName,
			})
		}

		if url, err := deps.FileStorage.SignedURL(r.Context(), attachment.StoragePath, time.Hour); err == nil {
			attachment.DownloadURL = url
		}

		writeJSON(w, http.StatusCreated, attachment)
	}
}

func deleteAttachmentHandler(deps RouterDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := auth.ViewerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "viewer is required")
			return
		}

		attachmentID := chi.URLParam(r, "attachmentID")
		if _, err := uuid.Parse(attachmentID); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "attachment id must be a valid UUID")
			return
		}

		attachment, err := deps.TaskStore.DeleteAttachment(r.Context(), attachmentID, viewer.UserID, false)
		if err != nil {
			writeTaskStoreError(w, err, "failed to delete attachment", "attachment not found")
			return
		}

		if deps.FileStorage != nil {
			_ = deps.FileStorage.Delete(r.Context(), attachment.StoragePath)
		}

		if deps.ActivityStore != nil {
			_ = deps.ActivityStore.Record(r.Context(), attachment.TaskID.String(), viewer, activity.ActionAttachmentRemoved, map[string]any{
				"file_name": attachment.FileName,
			})
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func writeTaskStoreError(w http.ResponseWriter, err error, internalMessage, notFoundMessage string) {
	if err == tasks.ErrNotFound {
		writeError(w, http.StatusNotFound, "not_found", notFoundMessage)
		return
	}
	log.Printf("%s: %v", internalMessage, err)
	writeError(w, http.StatusInternalServerError, "internal_error", internalMessage)
}
