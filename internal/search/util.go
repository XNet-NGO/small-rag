package search

import (
	"encoding/binary"
	"math"
)

// EncodeEmbedding encodes embedding to BLOB
func EncodeEmbedding(embedding []float32) []byte {
	data := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := math.Float32bits(v)
		binary.LittleEndian.PutUint32(data[i*4:i*4+4], bits)
	}
	return data
}

// DecodeEmbedding decodes embedding from BLOB
func DecodeEmbedding(data []byte) []float32 {
	if len(data)%4 != 0 {
		return []float32{}
	}

	embedding := make([]float32, len(data)/4)
	for i := 0; i < len(embedding); i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		embedding[i] = math.Float32frombits(bits)
	}

	return embedding
}

// CosineSimilarity calculates cosine similarity between two embeddings
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct float32 = 0
	var normA float32 = 0
	var normB float32 = 0

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / float32(math.Sqrt(float64(normA*normB)))
}
