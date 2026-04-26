package service

import "errors"

var (
	ErrTopicNotFound    = errors.New("topic_not_found")
	ErrTopicNotActive   = errors.New("topic_not_active")
	ErrNoActiveTopic    = errors.New("no_active_topic")
	ErrInvalidThreshold = errors.New("invalid_threshold")
	ErrQueueFull        = errors.New("queue_full")
	ErrNoLabelsToMerge  = errors.New("no_labels_to_merge")
)
