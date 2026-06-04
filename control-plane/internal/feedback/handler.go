package feedback

import (
	"context"

	"connectrpc.com/connect"

	feedbackv1 "github.com/harpia/control-plane/gen/harpia/feedback/v1"
)

type FeedbackHandler struct{}

func NewFeedbackHandler() *FeedbackHandler {
	return &FeedbackHandler{}
}

func (h *FeedbackHandler) RequestFeedback(ctx context.Context, req *connect.Request[feedbackv1.RequestFeedbackRequest]) (*connect.Response[feedbackv1.RequestFeedbackResponse], error) {
	return connect.NewResponse(&feedbackv1.RequestFeedbackResponse{
		FeedbackRequest: &feedbackv1.FeedbackRequest{
			Id:       "feedback-unknown",
			Status:   feedbackv1.FeedbackStatus_FEEDBACK_STATUS_PENDING,
			Question: req.Msg.Question,
		},
	}), nil
}

func (h *FeedbackHandler) SubmitFeedback(ctx context.Context, req *connect.Request[feedbackv1.SubmitFeedbackRequest]) (*connect.Response[feedbackv1.SubmitFeedbackResponse], error) {
	return connect.NewResponse(&feedbackv1.SubmitFeedbackResponse{
		FeedbackRequest: &feedbackv1.FeedbackRequest{
			Id:     req.Msg.FeedbackId,
			Status: feedbackv1.FeedbackStatus_FEEDBACK_STATUS_APPROVED,
		},
	}), nil
}

func (h *FeedbackHandler) ListPendingFeedback(ctx context.Context, req *connect.Request[feedbackv1.ListPendingFeedbackRequest], stream *connect.ServerStream[feedbackv1.ListPendingFeedbackResponse]) error {
	return nil
}

func (h *FeedbackHandler) GetFeedbackStatus(ctx context.Context, req *connect.Request[feedbackv1.GetFeedbackStatusRequest]) (*connect.Response[feedbackv1.GetFeedbackStatusResponse], error) {
	return connect.NewResponse(&feedbackv1.GetFeedbackStatusResponse{
		FeedbackRequest: &feedbackv1.FeedbackRequest{
			Id:       req.Msg.FeedbackId,
			Status:   feedbackv1.FeedbackStatus_FEEDBACK_STATUS_PENDING,
			TenantId: req.Msg.TenantId,
		},
	}), nil
}
