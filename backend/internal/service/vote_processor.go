package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/topic-voting/backend/internal/model"
)

type WSBroadcaster interface {
	BroadcastLeaderboard(topicID uuid.UUID)
	BroadcastChatMessage(topicID uuid.UUID, msg interface{})
}

type PendingVote struct {
	TopicID          uuid.UUID
	TopicTitle       string
	Username         string
	Message          string
	IsDonation       bool
	BitsAmount       int
	Threshold        float64
	VotingMode       string
	DonationAmount   float64
	DonationCurrency string
}

type ClassifiedVote struct {
	PendingVote   *PendingVote
	ClassifiedL   string
	Confidence    float64
	Vote          *model.Vote
}

type VoteProcessor struct {
	enqueueCh  chan *PendingVote
	resultCh   chan *ClassifiedVote
	workers    int
	classifier ClassifierClient
	tallyCache *VoteTallyCache
	wsHub      WSBroadcaster
	exchanger  Exchanger
	quit       chan struct{}
	wg         sync.WaitGroup
}

func NewVoteProcessor(
	queueCap int,
	workers int,
	classifier ClassifierClient,
	tallyCache *VoteTallyCache,
	wsHub WSBroadcaster,
	exchanger Exchanger,
) *VoteProcessor {
	return &VoteProcessor{
		enqueueCh:  make(chan *PendingVote, queueCap),
		resultCh:   make(chan *ClassifiedVote, queueCap),
		workers:    workers,
		classifier: classifier,
		tallyCache: tallyCache,
		wsHub:      wsHub,
		exchanger:  exchanger,
		quit:       make(chan struct{}),
	}
}

func (p *VoteProcessor) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	p.wg.Add(1)
	go p.resultConsumer()
}

func (p *VoteProcessor) Stop() {
	close(p.quit)
	p.wg.Wait()
}

func (p *VoteProcessor) Enqueue(vote *PendingVote) error {
	select {
	case p.enqueueCh <- vote:
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *VoteProcessor) worker() {
	defer p.wg.Done()
	for {
		select {
		case pv := <-p.enqueueCh:
			labels := p.tallyCache.GetLabels(pv.TopicID)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			clsResult, err := p.classifier.Classify(
				ctx, pv.Message, pv.TopicTitle, labels, pv.Threshold,
			)
			cancel()

			label := "uncategorized"
			confidence := 0.0
			if err != nil {
				log.Printf("[vote-processor] classify error for topic %s: %v", pv.TopicID, err)
			} else {
				label = clsResult.Label
				confidence = clsResult.Confidence
			}

			vote := &model.Vote{
				TopicID:         pv.TopicID,
				Username:        pv.Username,
				RawMessage:      pv.Message,
				ClassifiedLabel: label,
				Confidence:      confidence,
				Weight:          computeWeight(pv, p.exchanger),
				IsDonation:      pv.IsDonation,
				BitsAmount:      pv.BitsAmount,
				DonationAmount:  pv.DonationAmount,
				DonationCurrency: pv.DonationCurrency,
			}

			result := &ClassifiedVote{
				PendingVote: pv,
				ClassifiedL: label,
				Confidence:  confidence,
				Vote:        vote,
			}

			select {
			case p.resultCh <- result:
			case <-p.quit:
				return
			}

		case <-p.quit:
			return
		}
	}
}

func (p *VoteProcessor) resultConsumer() {
	defer p.wg.Done()
	for {
		select {
		case cv := <-p.resultCh:
			p.tallyCache.Increment(
				cv.Vote.TopicID,
				cv.PendingVote.TopicTitle,
				cv.PendingVote.VotingMode,
				cv.ClassifiedL,
				cv.Vote.Weight,
				cv.Vote,
			)
			topicID := cv.Vote.TopicID
			go p.wsHub.BroadcastLeaderboard(topicID)
			go p.wsHub.BroadcastChatMessage(topicID, &WSClassifiedMessage{
				Type: "chat_classified",
				Data: WSClassifiedData{
					Username:         cv.PendingVote.Username,
					Message:          cv.PendingVote.Message,
					ClassifiedLabel:  cv.ClassifiedL,
					Confidence:       cv.Confidence,
					Weight:           cv.Vote.Weight,
					DonationAmount:   cv.PendingVote.DonationAmount,
					DonationCurrency: cv.PendingVote.DonationCurrency,
				},
			})
		case <-p.quit:
			return
		}
	}
}

func computeWeight(pv *PendingVote, exchanger Exchanger) float64 {
	switch pv.VotingMode {
	case "donation":
		if !pv.IsDonation {
			return 0
		}
		if pv.DonationAmount > 0 {
			return exchanger.ToUSD(pv.DonationAmount, pv.DonationCurrency)
		}
		return float64(pv.BitsAmount) / 100.0
	default: // "chat"
		if !pv.IsDonation {
			return 1.0
		}
		if pv.DonationAmount > 0 {
			return 1.0 + pv.DonationAmount
		}
		return 1.0 + float64(pv.BitsAmount)/100.0
	}
}

type WSClassifiedMessage struct {
	Type string             `json:"type"`
	Data WSClassifiedData   `json:"data"`
}

type WSClassifiedData struct {
	Username         string  `json:"username"`
	Message          string  `json:"message"`
	ClassifiedLabel  string  `json:"classified_label"`
	Confidence       float64 `json:"confidence"`
	Weight           float64 `json:"weight"`
	DonationAmount   float64 `json:"donation_amount"`
	DonationCurrency string  `json:"donation_currency"`
}
