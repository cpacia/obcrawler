package rpc

import (
	"context"
	"github.com/cpacia/obcrawler/rpc/pb"
	"github.com/cpacia/openbazaar3.0/models"
	obpb "github.com/cpacia/openbazaar3.0/orders/pb"
	"github.com/golang/protobuf/ptypes"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("RPC")

// GrpcServer represents the server which implements the gRPC interface.
type GrpcServer struct {
	crawler Crawler
}

// NewGrpcServer returns new gRPC server implementation.
func NewGrpcServer(crawler Crawler) *GrpcServer {
	return &GrpcServer{
		crawler: crawler,
	}
}

// Subscribe is an RPC which streams new profiles and listings as they
// are crawled. Note you should subscribe to this as soon as the crawler
// starts as any data that is crawled while not subscribed will not be
// resent until the node is crawled again.
//
// Also, search engines MUST respect the expiration and not return any
// data which has expired.
func (s *GrpcServer) Subscribe(req *pb.SubscribeRequest, stream pb.Obcrawler_SubscribeServer) error {
	sub, err := s.crawler.Subscribe()
	if err != nil {
		return err
	}
	defer sub.Close()

	for {
		select {
		case obj := <-sub.Out:
			ts, err := ptypes.TimestampProto(obj.ExpirationDate)
			if err != nil {
				log.Errorf("Error creating expiration timestamp: %s", err)
				continue
			}
			switch o := obj.Data.(type) {
			case *models.Profile:
				lastModified, err := ptypes.TimestampProto(o.LastModified)
				if err != nil {
					log.Errorf("Error creating last modified timestamp: %s", err)
					continue
				}
				pro := &pb.UserData_Profile{
					Profile: &pb.Profile{
						PeerID:           o.PeerID,
						Name:             o.Name,
						Handle:           o.Handle,
						Location:         o.Location,
						About:            o.About,
						ShortDescription: o.ShortDescription,
						Nsfw:             o.Nsfw,
						Vendor:           o.Vendor,
						Moderator:        o.Moderator,
						Colors: &pb.Profile_ProfileColors{
							Primary:       o.Colors.Primary,
							Secondary:     o.Colors.Secondary,
							Text:          o.Colors.Text,
							Highlight:     o.Colors.Highlight,
							HighlightText: o.Colors.HighlightText,
						},
						AvatarHashes: &pb.Profile_ImageHashes{
							Tiny:     o.AvatarHashes.Tiny,
							Small:    o.AvatarHashes.Small,
							Medium:   o.AvatarHashes.Medium,
							Large:    o.AvatarHashes.Large,
							Original: o.AvatarHashes.Original,
							Filename: o.AvatarHashes.Filename,
						},
						HeaderHashes: &pb.Profile_ImageHashes{
							Tiny:     o.HeaderHashes.Tiny,
							Small:    o.HeaderHashes.Small,
							Medium:   o.HeaderHashes.Medium,
							Large:    o.HeaderHashes.Large,
							Original: o.HeaderHashes.Original,
							Filename: o.HeaderHashes.Filename,
						},
						PublicKey:              o.EscrowPublicKey,
						StoreAndForwardServers: o.StoreAndForwardServers,
						LastModified:           lastModified,
					},
				}

				if o.ModeratorInfo != nil {
					pro.Profile.ModeratorInfo = &pb.Profile_ModeratorInfo{
						Fee: &pb.Profile_ModeratorInfo_ModeratorFee{
							Percentage: float32(o.ModeratorInfo.Fee.Percentage),
						},
						Description:        o.ModeratorInfo.Description,
						AcceptedCurrencies: o.ModeratorInfo.AcceptedCurrencies,
						Languages:          o.ModeratorInfo.Languages,
						TermsAndConditions: o.ModeratorInfo.TermsAndConditions,
					}
					switch o.ModeratorInfo.Fee.FeeType {
					case models.FixedFee:
						pro.Profile.ModeratorInfo.Fee.FeeType = pb.Profile_ModeratorInfo_ModeratorFee_FixedFee
					case models.PercentageFee:
						pro.Profile.ModeratorInfo.Fee.FeeType = pb.Profile_ModeratorInfo_ModeratorFee_PercentageFee
					case models.FixedPlusPercentageFee:
						pro.Profile.ModeratorInfo.Fee.FeeType = pb.Profile_ModeratorInfo_ModeratorFee_FixedPlusPercentageFee
					}
					if o.ModeratorInfo.Fee.FixedFee != nil {
						pro.Profile.ModeratorInfo.Fee.FixedFee = &pb.Profile_CurrencyValue{
							Amount: o.ModeratorInfo.Fee.FixedFee.Amount.String(),
						}
						if o.ModeratorInfo.Fee.FixedFee.Currency != nil {
							pro.Profile.ModeratorInfo.Fee.FixedFee.Currency = &pb.Profile_Currency{
								Code:         o.ModeratorInfo.Fee.FixedFee.Currency.Code.String(),
								Divisibility: uint32(o.ModeratorInfo.Fee.FixedFee.Currency.Divisibility),
							}
						}
					}
				}

				if o.ContactInfo != nil {
					pro.Profile.ContactInfo = &pb.Profile_ContactInfo{
						Email:       o.ContactInfo.Email,
						PhoneNumber: o.ContactInfo.PhoneNumber,
						Website:     o.ContactInfo.Website,
					}
					for _, s := range o.ContactInfo.Social {
						pro.Profile.ContactInfo.Social = append(pro.Profile.ContactInfo.Social, &pb.Profile_ContactInfo_SocialAccount{
							Type:     s.Type,
							Username: s.Username,
							Proof:    s.Proof,
						})
					}
				}

				if o.Stats != nil {
					pro.Profile.Stats = &pb.Profile_ProfileStats{
						FollowerCount:  o.Stats.FollowerCount,
						FollowingCount: o.Stats.FollowingCount,
						ListingCount:   o.Stats.ListingCount,
						PostCount:      o.Stats.PostCount,
						RatingCount:    o.Stats.RatingCount,
						AverageRating:  o.Stats.AverageRating,
					}
				}

				ud := &pb.UserData{
					Expiration: ts,
					Data:       pro,
				}

				if err := stream.Send(ud); err != nil {
					return err
				}
			case *obpb.SignedListing:
				ud := &pb.UserData{
					Expiration: ts,
					Data: &pb.UserData_Listing{
						Listing: o,
					},
				}
				if err := stream.Send(ud); err != nil {
					return err
				}
			}
		case <-stream.Context().Done():
			return nil // client disconnected
		}
	}
}

// CrawlNode queues up a crawl of the given node.
func (s *GrpcServer) CrawlNode(ctx context.Context, req *pb.CrawlNodeRequest) (*pb.CrawlNodeResponse, error) {
	pid, err := peer.IDB58Decode(req.Peer)
	if err != nil {
		return nil, err
	}
	return &pb.CrawlNodeResponse{}, s.crawler.CrawlNode(pid)
}

// BanNode will prevent the node from being crawled in the future as
// well as purge all cached/pinned files of this node from the crawler.
func (s *GrpcServer) BanNode(ctx context.Context, req *pb.BanNodeRequest) (*pb.BanNodeResponse, error) {
	pid, err := peer.IDB58Decode(req.Peer)
	if err != nil {
		return nil, err
	}
	return &pb.BanNodeResponse{}, s.crawler.BanNode(pid)
}

// UnbanNode will un-ban the provided node. It will not immediately
// crawl the node again. If you want that call CrawlNode.
func (s *GrpcServer) UnbanNode(ctx context.Context, req *pb.UnbanNodeRequest) (*pb.UnbanNodeResponse, error) {
	pid, err := peer.IDB58Decode(req.Peer)
	if err != nil {
		return nil, err
	}
	return &pb.UnbanNodeResponse{}, s.crawler.UnbanNode(pid)
}
