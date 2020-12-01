package quantile

import (
	"errors"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/sketches-go/ddsketch/mapping"
	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
)

type ddSketch struct {
	bins []float64
	offset int
	zeros int
	mapping mapping.IndexMapping
}

func (s *ddSketch) get(index int) int {
	if index < s.offset || index >= s.offset + len(s.bins) {
		return 0
	}
	return int(s.bins[index-s.offset])
}

// decodeDDSketch decodes a ddSketch from a protobuf encoded ddSketch
// it only supports positive contiguous bins
func decodeDDSketch(data []byte) (ddSketch, error) {
	var sketchPb pb.DDSketch
	if err := proto.Unmarshal(data, &sketchPb); err != nil {
		return ddSketch{}, err
	}
	mapping, err := getDDSketchMapping(sketchPb.Mapping)
	if err != nil {
		return ddSketch{}, err
	}
	if sketchPb.Mapping.IndexOffset > 0 ||
		len(sketchPb.NegativeValues.BinCounts) > 0 ||
		len(sketchPb.NegativeValues.ContiguousBinCounts) > 0 ||
		len(sketchPb.PositiveValues.BinCounts) > 0 {
		return ddSketch{}, errors.New("ddSketch format not supported")
	}
	return ddSketch{
		mapping: mapping,
		bins: sketchPb.PositiveValues.ContiguousBinCounts,
		offset: int(sketchPb.PositiveValues.ContiguousBinIndexOffset),
		zeros: int(sketchPb.ZeroCount),
	}, nil
}

func getDDSketchMapping(protoMapping *pb.IndexMapping) (m mapping.IndexMapping, err error) {
	switch protoMapping.Interpolation {
	case pb.IndexMapping_NONE:
		return mapping.NewLogarithmicMappingWithGamma(protoMapping.Gamma, protoMapping.IndexOffset)
	case pb.IndexMapping_LINEAR:
		return mapping.NewLinearlyInterpolatedMappingWithGamma(protoMapping.Gamma, protoMapping.IndexOffset)
	case pb.IndexMapping_CUBIC:
		return mapping.NewCubicallyInterpolatedMappingWithGamma(protoMapping.Gamma, protoMapping.IndexOffset)
	default:
		return nil, fmt.Errorf("interpolation not supported: %d", protoMapping.Interpolation)
	}
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// DDToGKSketches converts two dd sketches: ok and errors to 2 gk sketches: hits and errors
// with hits = ok + errors
func DDToGKSketches(okSketchData []byte, errSketchData []byte) (hits, errors *SliceSummary, err error) {
	okDDSketch, err := decodeDDSketch(okSketchData)
	if err != nil {
		return nil, nil, err
	}
	// todo: remove dump
	fmt.Println("\nok sketch")
	spew.Dump(okDDSketch)
	errDDSketch, err := decodeDDSketch(errSketchData)
	if err != nil {
		return nil, nil, err
	}
	// todo: remove dump
	fmt.Println("\nerror sketch")
	spew.Dump(errDDSketch)

	minOffset := min(okDDSketch.offset, errDDSketch.offset)
	maxIndex := max(okDDSketch.offset + len(okDDSketch.bins), errDDSketch.offset + len(errDDSketch.bins))
	hits = &SliceSummary{Entries: make([]Entry, 0, maxIndex - minOffset)}
	errors = &SliceSummary{Entries: make([]Entry, 0, len(errDDSketch.bins))}
	if zeros := okDDSketch.zeros + errDDSketch.zeros; zeros > 0 {
		hits.Entries = append(hits.Entries, Entry{V: 0, G: zeros, Delta: 0})
		hits.N = zeros
	}
	if zeros := errDDSketch.zeros; zeros > 0 {
		errors.Entries = append(errors.Entries, Entry{V: 0, G: zeros, Delta: 0})
		errors.N = zeros
	}
	for index := minOffset; index < maxIndex; index++ {
		gErr := errDDSketch.get(index)
		gHits := okDDSketch.get(index) + gErr
		if gHits == 0 {
			// gHits == 0 implies gErr == 0
			continue
		}
		hits.N += gHits
		v := okDDSketch.mapping.Value(index)
		hits.Entries = append(hits.Entries, Entry{
			V:     v,
			G:     gHits,
			Delta: int(2 * EPSILON * float64(hits.N-1)),
		})
		if gErr == 0 {
			continue
		}
		errors.N += gErr
		errors.Entries = append(errors.Entries, Entry{
			V:     v,
			G:     gErr,
			Delta: int(2 * EPSILON * float64(errors.N-1)),
		})
	}
	if hits.N > 0 {
		hits.Entries[0].Delta = 0
		hits.Entries[len(hits.Entries)-1].Delta = 0
	}
	if errors.N > 0 {
		errors.Entries[0].Delta = 0
		errors.Entries[len(errors.Entries)-1].Delta = 0
	}
	hits.compress()
	errors.compress()
	return hits, errors, nil
}
