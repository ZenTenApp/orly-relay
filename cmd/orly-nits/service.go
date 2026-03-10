package main

import (
	"context"

	orlynitsv1 "next.orly.dev/pkg/proto/orlynits/v1"
)

// NitsService implements the gRPC NitsService interface.
type NitsService struct {
	orlynitsv1.UnimplementedNitsServiceServer
	node *Bitcoind
}

func NewNitsService(node *Bitcoind) *NitsService {
	return &NitsService{node: node}
}

func (s *NitsService) Ready(_ context.Context, _ *orlynitsv1.Empty) (*orlynitsv1.ReadyResponse, error) {
	if !s.node.IsResponsive() {
		return &orlynitsv1.ReadyResponse{
			Ready:  false,
			Status: "starting",
		}, nil
	}

	info := s.node.GetBlockchainInfo()
	if info == nil {
		return &orlynitsv1.ReadyResponse{
			Ready:  false,
			Status: "starting",
		}, nil
	}

	progress := int32(info.VerificationProgress * 100)

	if info.InitialBlockDownload {
		return &orlynitsv1.ReadyResponse{
			Ready:                false,
			Status:               "syncing",
			SyncProgressPercent:  progress,
		}, nil
	}

	return &orlynitsv1.ReadyResponse{
		Ready:                true,
		Status:               "ready",
		SyncProgressPercent:  progress,
	}, nil
}

func (s *NitsService) GetBlockchainInfo(_ context.Context, _ *orlynitsv1.Empty) (*orlynitsv1.BlockchainInfoResponse, error) {
	info := s.node.GetBlockchainInfo()
	if info == nil {
		return &orlynitsv1.BlockchainInfoResponse{}, nil
	}

	return &orlynitsv1.BlockchainInfoResponse{
		Chain:                 info.Chain,
		Blocks:                info.Blocks,
		Headers:               info.Headers,
		VerificationProgress:  info.VerificationProgress,
		InitialBlockDownload:  info.InitialBlockDownload,
		Pruned:                info.Pruned,
		PruneHeight:           info.PruneHeight,
		SizeOnDisk:            info.SizeOnDisk,
	}, nil
}

func (s *NitsService) EstimateFee(_ context.Context, req *orlynitsv1.EstimateFeeRequest) (*orlynitsv1.EstimateFeeResponse, error) {
	if !s.node.IsResponsive() {
		return &orlynitsv1.EstimateFeeResponse{
			Errors: []string{"bitcoind is not responsive"},
		}, nil
	}

	confTarget := int(req.ConfTarget)
	if confTarget < 1 {
		confTarget = 6
	}

	result, err := s.node.EstimateFee(confTarget)
	if err != nil {
		return &orlynitsv1.EstimateFeeResponse{
			Errors: []string{err.Error()},
		}, nil
	}

	return &orlynitsv1.EstimateFeeResponse{
		FeeRate: result.FeeRate,
		Errors:  result.Errors,
	}, nil
}
