// Copyright 2020 Anapaya Systems
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/seg"
	"github.com/scionproto/scion/go/lib/pathdb/query"
	"github.com/scionproto/scion/go/lib/scrypto"
	"github.com/scionproto/scion/go/lib/scrypto/cppki"
	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/pkg/api"
	cstrust "github.com/scionproto/scion/go/pkg/cs/trust"
)

type SegmentsStore interface {
	Get(context.Context, *query.Params) (query.Results, error)
}

// Server implements the Control Service API.
type Server struct {
	Segments SegmentsStore
	CA       cstrust.ChainBuilder
	Config   http.HandlerFunc
	Info     http.HandlerFunc
	LogLevel http.HandlerFunc
	Signer   cstrust.RenewingSigner
	Topology http.HandlerFunc
}

// GetSegments gets the stored in the PathDB.
func (s *Server) GetSegments(w http.ResponseWriter, r *http.Request, params GetSegmentsParams) {
	q := query.Params{}
	var errs serrors.List
	if params.StartIsdAs != nil {
		if ia, err := addr.IAFromString(string(*params.StartIsdAs)); err == nil {
			q.StartsAt = []addr.IA{ia}
		} else {
			errs = append(errs, serrors.WithCtx(err, "parameter", "start_isd_as"))
		}
	}
	if params.EndIsdAs != nil {
		if ia, err := addr.IAFromString(string(*params.EndIsdAs)); err == nil {
			q.EndsAt = []addr.IA{ia}
		} else {
			errs = append(errs, serrors.WithCtx(err, "parameter", "end_isd_as"))
		}
	}
	if err := errs.ToError(); err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusBadRequest,
			Title:  "malformed query parameters",
			Type:   api.StringRef(api.BadRequest),
		})
		return
	}
	res, err := s.Segments.Get(r.Context(), &q)
	if err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusInternalServerError,
			Title:  "error getting segments",
			Type:   api.StringRef(api.InternalError),
		})
		return
	}
	sort.Sort(query.Results(res))
	rep := make([]*SegmentBrief, 0, len(res))
	for _, segRes := range res {
		rep = append(rep, &SegmentBrief{
			Id:         SegmentID(segID(segRes.Seg)),
			StartIsdAs: IsdAs(segRes.Seg.FirstIA().String()),
			EndIsdAs:   IsdAs(segRes.Seg.LastIA().String()),
			Length:     len(segRes.Seg.ASEntries),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	if err := enc.Encode(rep); err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusInternalServerError,
			Title:  "unable to marshal response",
			Type:   api.StringRef(api.InternalError),
		})
		return
	}
}

// GetSegment gets a segments details specified by its ID.
func (s *Server) GetSegment(w http.ResponseWriter, r *http.Request, segmentId SegmentIDs) {
	ids, err := decodeSegmentIDs(segmentId)
	if err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusBadRequest,
			Title:  "malformed query parameters",
			Type:   api.StringRef(api.BadRequest),
		})
		return
	}
	resp, err := s.getSegmentsByID(r.Context(), ids)
	if err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusInternalServerError,
			Title:  "error getting segments",
			Type:   api.StringRef(api.InternalError),
		})
		return
	}
	rep := make([]*Segment, 0, len(resp))
	for _, segRes := range resp {
		var hops []Hop
		for i, as := range segRes.Seg.ASEntries {
			if i != 0 {
				hops = append(hops, Hop{
					Interface: int(as.HopEntry.HopField.ConsIngress),
					IsdAs:     IsdAs(as.Local.String())})
			}
			if i != len(segRes.Seg.ASEntries)-1 {
				hops = append(hops, Hop{
					Interface: int(as.HopEntry.HopField.ConsEgress),
					IsdAs:     IsdAs(as.Local.String())})
			}
		}
		rep = append(rep, &Segment{
			Id:          SegmentID(segID(segRes.Seg)),
			Timestamp:   segRes.Seg.Info.Timestamp.UTC(),
			Expiration:  segRes.Seg.MinExpiry().UTC(),
			LastUpdated: segRes.LastUpdate.UTC(),
			Hops:        hops,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	if err := enc.Encode(rep); err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusInternalServerError,
			Title:  "unable to marshal response",
			Type:   api.StringRef(api.InternalError),
		})
		return
	}
}

// GetSegmentBlob gets a segment (specified by its ID) as a pem encoded blob.
func (s *Server) GetSegmentBlob(w http.ResponseWriter, r *http.Request, segmentId SegmentIDs) {
	ids, err := decodeSegmentIDs(segmentId)
	if err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusBadRequest,
			Title:  "malformed query parameters",
			Type:   api.StringRef(api.BadRequest),
		})
		return
	}
	resp, err := s.getSegmentsByID(r.Context(), ids)
	if err != nil {
		Error(w, Problem{
			Detail: api.StringRef(err.Error()),
			Status: http.StatusInternalServerError,
			Title:  "error getting segments",
			Type:   api.StringRef(api.InternalError),
		})
		return
	}
	var buf bytes.Buffer
	for _, segRes := range resp {
		bytes, err := proto.Marshal(seg.PathSegmentToPB(segRes.Seg))
		if err != nil {
			Error(w, Problem{
				Detail: api.StringRef(err.Error()),
				Status: http.StatusInternalServerError,
				Title:  "unable to marshal segment",
				Type:   api.StringRef(api.InternalError),
			})
			return
		}
		b := &pem.Block{
			Type:  "PATH SEGMENT",
			Bytes: bytes,
		}
		if err := pem.Encode(&buf, b); err != nil {
			Error(w, Problem{
				Detail: api.StringRef(err.Error()),
				Status: http.StatusInternalServerError,
				Title:  "unable to marshal response",
				Type:   api.StringRef(api.InternalError),
			})
			return
		}
	}
	io.Copy(w, &buf)
}

// GetCa gets the CA info
func (s *Server) GetCa(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	p, err := s.CA.PolicyGen.Generate(r.Context())
	if err != nil {
		http.Error(w, "No active signer", http.StatusInternalServerError)
		return
	}

	ia, err := cppki.ExtractIA(p.Certificate.Subject)
	if err != nil || ia == nil {
		http.Error(w, "Unable to get extract ISD-AS", http.StatusInternalServerError)
		return
	}

	type Subject struct {
		IA addr.IA `json:"isd_as"`
	}
	type Validity struct {
		NotBefore time.Time `json:"not_before"`
		NotAfter  time.Time `json:"not_after"`
	}
	type Policy struct {
		ChainLifetime string `json:"chain_lifetime"`
	}
	rep := struct {
		Subject      Subject  `json:"subject"`
		SubjectKeyID string   `json:"subject_key_id"`
		Policy       Policy   `json:"policy"`
		CertValidity Validity `json:"cert_validity"`
	}{
		Subject:      Subject{IA: *ia},
		SubjectKeyID: fmt.Sprintf("% X", p.Certificate.SubjectKeyId),
		Policy: Policy{
			ChainLifetime: fmt.Sprintf("%s", p.Validity),
		},
		CertValidity: Validity{
			NotBefore: p.Certificate.NotBefore,
			NotAfter:  p.Certificate.NotAfter,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	if err := enc.Encode(rep); err != nil {
		http.Error(w, "Unable to marshal response", http.StatusInternalServerError)
		return
	}
}

// GetConfig is an indirection to the http handler.
func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.Config(w, r)
}

// GetInfo is an indirection to the http handler.
func (s *Server) GetInfo(w http.ResponseWriter, r *http.Request) {
	s.Info(w, r)
}

// GetLogLevel is an indirection to the http handler.
func (s *Server) GetLogLevel(w http.ResponseWriter, r *http.Request) {
	s.LogLevel(w, r)
}

// SetLogLevel is an indirection to the http handler.
func (s *Server) SetLogLevel(w http.ResponseWriter, r *http.Request) {
	s.LogLevel(w, r)
}

// GetSigner  generates the singer response content.
func (s *Server) GetSigner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	p, err := s.Signer.SignerGen.Generate(r.Context())
	if err != nil {
		http.Error(w, "Unable to get signer", http.StatusInternalServerError)
		return
	}

	type Subject struct {
		IA addr.IA `json:"isd_as"`
	}
	type TRCID struct {
		ISD    addr.ISD        `json:"isd"`
		Base   scrypto.Version `json:"base_number"`
		Serial scrypto.Version `json:"serial_number"`
	}
	type Validity struct {
		NotBefore time.Time `json:"not_before"`
		NotAfter  time.Time `json:"not_after"`
	}
	rep := struct {
		Subject       Subject   `json:"subject"`
		SubjectKeyID  string    `json:"subject_key_id"`
		Expiration    time.Time `json:"expiration"`
		TRCID         TRCID     `json:"trc_id"`
		ChainValidity Validity  `json:"chain_validity"`
		InGrace       bool      `json:"in_grace_period"`
	}{
		Subject:      Subject{IA: p.IA},
		SubjectKeyID: fmt.Sprintf("% X", p.SubjectKeyID),
		Expiration:   p.Expiration,
		TRCID: TRCID{
			ISD:    p.TRCID.ISD,
			Base:   p.TRCID.Base,
			Serial: p.TRCID.Serial,
		},
		ChainValidity: Validity{
			NotBefore: p.ChainValidity.NotBefore,
			NotAfter:  p.ChainValidity.NotAfter,
		},
		InGrace: p.InGrace,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	if err := enc.Encode(rep); err != nil {
		http.Error(w, "Unable to marshal response", http.StatusInternalServerError)
		return
	}
}

// GetTopology is an indirection to the http handler.
func (s *Server) GetTopology(w http.ResponseWriter, r *http.Request) {
	s.Topology(w, r)
}

// Error creates an detailed error response.
func Error(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(int(p.Status))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	// no point in catching error here, there is nothing we can do about it anymore.
	enc.Encode(p)
}

// segID makes a hex encoded string of the segment id.
func segID(s *seg.PathSegment) string { return fmt.Sprintf("%x", s.ID()) }

// getSegmentsByID requests the segments and Sort the result according to the requested order.
func (s *Server) getSegmentsByID(ctx context.Context,
	ids []common.RawBytes) (query.Results, error) {
	q := query.Params{SegIDs: ids}
	r, err := s.Segments.Get(ctx, &q)
	for i, id := range ids {
		for j := i; j < len(r); j++ {
			if segID(r[j].Seg) == string(id) {
				r.Swap(i, j)
				break
			}
		}
	}
	return r, err
}

// decodeSegmentIDs converts segment IDs to RawBytes.
func decodeSegmentIDs(ids SegmentIDs) ([]common.RawBytes, error) {
	b := make([]common.RawBytes, 0, len(ids))
	var errs serrors.List
	for _, segID := range ids {
		if id, err := hex.DecodeString(string(segID)); err == nil {
			b = append(b, id)
		} else {
			errs = append(errs, serrors.WithCtx(err, "parameter", "id"))
		}
	}
	if err := errs.ToError(); err != nil {
		return nil, err
	}
	return b, nil
}
