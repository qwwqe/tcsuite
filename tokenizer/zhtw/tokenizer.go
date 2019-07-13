package zhtw

import (
	"errors"
	//"fmt"
	"github.com/qwwqe/tcsuite/entities/corpus"
	"github.com/qwwqe/tcsuite/lexicon"
	"github.com/qwwqe/tcsuite/tokenizer"
	"math"
	"unicode/utf8"
)

type zhtwTokenizer struct {
	Options *tokenizer.Options
}

func NewTokenizer(options *tokenizer.Options) tokenizer.Interface {
	return &zhtwTokenizer{
		Options: options,
	}
}

type segNode struct {
	segString       string
	freq            int
	depth           int
	cumulativeRunes int
	numRunes        int
	textOffset      int
	parent          *segNode
}

func (t *zhtwTokenizer) Tokenize(text string, l lexicon.Lexicon) ([]*corpus.Word, error) {
	words := []*corpus.Word{}

	// Check for UTF-8 validity
	if !utf8.ValidString(text) {
		return []*corpus.Word{}, errors.New("Tokenizer: Invalid UTF-8 sequence.")
	}

	textOffset := 0
	for textOffset < len(text) {
		// Find leading lexical items
		rootSegment := &segNode{
			depth:      0,
			textOffset: textOffset,
		}
		leadingSegments, failureOffset := findAllFollowingSegments(text, rootSegment, l)

		// If no leading segments exist, there must be a leading string of non-lexical characters,
		// so clump these together into a single segment and jump ahead
		if len(leadingSegments) == 0 {
			if failureOffset == textOffset { // This should not happen
				break
			}

			nonLexLeader := &corpus.Word{
				Word:    text[textOffset:failureOffset],
				Lexical: false,
			}
			words = append(words, nonLexLeader)
			textOffset = failureOffset
			continue
		}

		// Select candidates based on longest total length
		candidates := []*segNode{}
		segments := leadingSegments
		for len(segments) > 0 {
			segment := segments[0]
			segments = segments[1:]

			// Consider addition of segment to candidates
			if len(candidates) == 0 || segment.cumulativeRunes > candidates[0].cumulativeRunes {
				candidates = []*segNode{segment}
			} else if segment.cumulativeRunes == candidates[0].cumulativeRunes {
				candidates = append(candidates, segment)
			}

			// If depth not exceeded, get following segments and add to queue
			if segment.depth < t.Options.MaxDepth {
				nextSegments, _ := findAllFollowingSegments(text, segment, l)
				if len(nextSegments) > 0 {
					segments = append(segments, nextSegments...)
				}
			}
		}

		candidates = filterByGreatestAverageLength(candidates)
		candidates = filterBySmallestWordLengthVariance(candidates)
		candidates = filterByLargestSumOfSingleCharFrequency(candidates)

		if len(candidates) == 0 {
			return []*corpus.Word{}, errors.New("Tokenizer: No candidates, aborting tokenization.")
		}

		// Add segments (in reverse order) to the corpus and adjust textOffset
		finalCandidate := candidates[0]
		textOffset = finalCandidate.textOffset
		for i := finalCandidate.depth; i > 0; i-- {
			words = append(words, nil)
		}
		for i, segment := len(words)-1, finalCandidate; segment != nil && segment.depth != 0; i, segment = i-1, segment.parent {
			words[i] = &corpus.Word{
				Word:    segment.segString,
				Lexical: true,
			}
		}
	}

	return words, nil
}

// findAllFollowingSegments returns all lexical entries immediately following that indicated by the provided segment.
// The integer return value indicates the text offset at which the search failed decisively.
func findAllFollowingSegments(text string, segment *segNode, l lexicon.Lexicon) ([]*segNode, int) {
	segments := []*segNode{}

	baseOffset := segment.textOffset
	totalWidth := 0
	totalRunes := 0
	for baseOffset+totalWidth < len(text) {
		_, width := utf8.DecodeRuneInString(text[baseOffset+totalWidth:])
		if width == 0 {
			break
		}
		totalWidth += width
		totalRunes += 1
		segString := text[baseOffset : baseOffset+totalWidth]
		freq, isPrefix, exists := l.GetLexemeFrequency(segString)
		if exists {
			newSegment := &segNode{
				segString:       segString,
				freq:            freq,
				depth:           segment.depth + 1,
				cumulativeRunes: segment.cumulativeRunes + totalRunes,
				numRunes:        totalRunes,
				textOffset:      baseOffset + totalWidth,
				parent:          segment,
			}
			segments = append(segments, newSegment)
		}

		if !exists && !isPrefix {
			break
		}
	}

	return segments, baseOffset + totalWidth
}

// filterByGreatestAverageLength filters candidates by greatest average word length.
func filterByGreatestAverageLength(candidates []*segNode) []*segNode {
	filteredCandidates := []*segNode{}
	maxMean := -1.0
	for _, candidate := range candidates {
		m := float64(candidate.cumulativeRunes) / float64(candidate.depth)
		if len(filteredCandidates) == 0 || m > maxMean {
			filteredCandidates = []*segNode{candidate}
			maxMean = m
		} else if m == maxMean {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}
	return filteredCandidates
}

// filterBySmallestWordLengthVariance filters candidates by smallest word length variance.
func filterBySmallestWordLengthVariance(candidates []*segNode) []*segNode {
	filteredCandidates := []*segNode{}
	leastVariance := math.MaxFloat64
	for _, candidate := range candidates {
		m := float64(candidate.cumulativeRunes) / float64(candidate.depth)
		squaredDifferenceSum := 0.0
		for segment := candidate; segment != nil && segment.depth != 0; segment = segment.parent {
			squaredDifferenceSum += math.Pow(float64(segment.numRunes)-m, 2)
		}

		variance := squaredDifferenceSum / float64(candidate.depth)

		if len(filteredCandidates) == 0 || variance < leastVariance {
			filteredCandidates = []*segNode{candidate}
			leastVariance = variance
		} else if variance == leastVariance {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}

	return filteredCandidates
}

// filterByLargestSumOfSingleCharFrequency filters candidates by largest sum of single-character word frequencies.
func filterByLargestSumOfSingleCharFrequency(candidates []*segNode) []*segNode {
	filteredCandidates := []*segNode{}
	maxSum := -1
	for _, candidate := range candidates {
		sum := 0
		for segment := candidate; segment != nil && segment.depth != 0; segment = segment.parent {
			if segment.numRunes == 1 {
				sum += segment.freq
			}
		}

		if len(filteredCandidates) == 0 || sum > maxSum {
			filteredCandidates = []*segNode{candidate}
		} else if sum == maxSum {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}

	return filteredCandidates
}
