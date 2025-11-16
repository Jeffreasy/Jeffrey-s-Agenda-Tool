// Package gmail handles Gmail-related background tasks.
package gmail

import (
	"agenda-automator-api/internal/domain"
	"context"
	"encoding/json"
	"log"

	"google.golang.org/api/gmail/v1"
)

// Action implementations.
func (gp *GmailProcessor) executeAutoReply(
	_ context.Context,
	srv *gmail.Service,
	acc domain.ConnectedAccount,
	message *gmail.Message,
	rule domain.GmailAutomationRule,
) error {
	var params struct {
		ReplyText string `json:"reply_text"`
	}
	if err := json.Unmarshal(rule.ActionParams, &params); err != nil {
		return err
	}

	reply := &gmail.Message{
		ThreadId: message.ThreadId,
		Raw:      gp.createReplyRaw(message, params.ReplyText, acc.Email),
	}

	_, err := srv.Users.Messages.Send("me", reply).Do()
	return err
}

func (gp *GmailProcessor) executeAddLabel(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	rule domain.GmailAutomationRule,
) error {
	var params struct {
		LabelName string `json:"label_name"`
	}
	if err := json.Unmarshal(rule.ActionParams, &params); err != nil {
		return err
	}

	label, err := gp.getOrCreateLabel(srv, params.LabelName)
	if err != nil {
		return err
	}

	modifyRequest := &gmail.ModifyMessageRequest{
		AddLabelIds: []string{label.Id},
	}

	_, err = srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

func (gp *GmailProcessor) executeRemoveLabel(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	rule domain.GmailAutomationRule,
) error {
	var params struct {
		LabelName string `json:"label_name"`
	}
	if err := json.Unmarshal(rule.ActionParams, &params); err != nil {
		return err
	}

	label, err := gp.getLabelByName(srv, params.LabelName)
	if err != nil {
		return err
	}

	modifyRequest := &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{label.Id},
	}

	_, err = srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

func (gp *GmailProcessor) executeMarkRead(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	_ domain.GmailAutomationRule,
) error {
	modifyRequest := &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}
	_, err := srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

func (gp *GmailProcessor) executeMarkUnread(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	_ domain.GmailAutomationRule,
) error {
	modifyRequest := &gmail.ModifyMessageRequest{
		AddLabelIds: []string{"UNREAD"},
	}
	_, err := srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

func (gp *GmailProcessor) executeArchive(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	_ domain.GmailAutomationRule,
) error {
	modifyRequest := &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"INBOX"},
	}
	_, err := srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

func (gp *GmailProcessor) executeTrash(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	_ domain.GmailAutomationRule,
) error {
	modifyRequest := &gmail.ModifyMessageRequest{
		AddLabelIds: []string{"TRASH"},
	}
	_, err := srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

func (gp *GmailProcessor) executeStar(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	_ domain.GmailAutomationRule,
) error {
	modifyRequest := &gmail.ModifyMessageRequest{
		AddLabelIds: []string{"STARRED"},
	}
	_, err := srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

func (gp *GmailProcessor) executeUnstar(
	_ context.Context,
	srv *gmail.Service,
	_ domain.ConnectedAccount,
	message *gmail.Message,
	_ domain.GmailAutomationRule,
) error {
	modifyRequest := &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"STARRED"},
	}
	_, err := srv.Users.Messages.Modify("me", message.Id, modifyRequest).Do()
	return err
}

// Simplified implementations.
func (gp *GmailProcessor) executeForward(
	_ context.Context,
	_ *gmail.Service,
	_ domain.ConnectedAccount,
	_ *gmail.Message,
	_ domain.GmailAutomationRule,
) error {
	log.Printf("[Gmail] Forward action not yet implemented")
	return nil
}
