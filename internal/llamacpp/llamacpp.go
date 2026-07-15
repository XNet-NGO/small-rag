package llamacpp

/*
#cgo CFLAGS: -I${SRCDIR}/../../llama/include
#cgo LDFLAGS: -L${SRCDIR}/../../llama/lib -lllama -lggml -lggml-cpu -lggml-base -lm -lstdc++ -lpthread -lgomp
#include <llama.h>
#include <stdlib.h>
#include <string.h>

// Helper to create context params with embedding enabled
static struct llama_context_params make_embed_ctx_params(int n_ctx, int n_threads) {
    struct llama_context_params params = llama_context_default_params();
    params.n_ctx = n_ctx;
    params.n_batch = n_ctx;
    params.n_threads = n_threads;
    params.n_threads_batch = n_threads;
    params.embeddings = true;
    params.pooling_type = LLAMA_POOLING_TYPE_UNSPECIFIED; // use model default
    return params;
}

// Helper to create model params
static struct llama_model_params make_model_params(void) {
    struct llama_model_params params = llama_model_default_params();
    return params;
}
*/
import "C"
import (
	"fmt"
	"math"
	"sync"
	"unsafe"
)

// Model wraps a llama.cpp model and context for embedding generation
type Model struct {
	model *C.struct_llama_model
	ctx   *C.struct_llama_context
	vocab *C.struct_llama_vocab
	nEmbd int
	mu    sync.Mutex
}

func init() {
	C.llama_backend_init()
}

// LoadModel loads a GGUF model for embedding generation
func LoadModel(path string, nCtx int, nThreads int) (*Model, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// Load model
	mparams := C.make_model_params()
	model := C.llama_model_load_from_file(cPath, mparams)
	if model == nil {
		return nil, fmt.Errorf("failed to load model: %s", path)
	}

	// Create context with embedding enabled
	cparams := C.make_embed_ctx_params(C.int(nCtx), C.int(nThreads))
	ctx := C.llama_init_from_model(model, cparams)
	if ctx == nil {
		C.llama_model_free(model)
		return nil, fmt.Errorf("failed to create context")
	}

	nEmbd := int(C.llama_model_n_embd(model))
	vocab := C.llama_model_get_vocab(model)

	return &Model{
		model: model,
		ctx:   ctx,
		vocab: vocab,
		nEmbd: nEmbd,
	}, nil
}

// EmbeddingDims returns the embedding dimension count
func (m *Model) EmbeddingDims() int {
	return m.nEmbd
}

// Embed generates a normalized embedding vector for the given text
func (m *Model) Embed(text string) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(text) == 0 {
		return make([]float32, m.nEmbd), nil
	}

	// Tokenize
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	textLen := C.int(len(text))

	// First call to get token count
	maxTokens := C.int(512)
	tokens := make([]C.llama_token, maxTokens)

	nTokens := C.llama_tokenize(
		m.vocab,
		cText,
		textLen,
		&tokens[0],
		maxTokens,
		C.bool(true),  // add_special
		C.bool(true),  // parse_special
	)

	if nTokens < 0 {
		// Need more space
		maxTokens = -nTokens
		tokens = make([]C.llama_token, maxTokens)
		nTokens = C.llama_tokenize(
			m.vocab,
			cText,
			textLen,
			&tokens[0],
			maxTokens,
			C.bool(true),
			C.bool(true),
		)
		if nTokens < 0 {
			return nil, fmt.Errorf("tokenization failed")
		}
	}

	// Create batch with sequence info for pooling
	batch := C.llama_batch_init(C.int(nTokens), 0, 1)
	batch.n_tokens = nTokens
	for i := C.int32_t(0); i < nTokens; i++ {
		*(*C.llama_token)(unsafe.Pointer(uintptr(unsafe.Pointer(batch.token)) + uintptr(i)*unsafe.Sizeof(C.llama_token(0)))) = tokens[i]
		*(*C.llama_pos)(unsafe.Pointer(uintptr(unsafe.Pointer(batch.pos)) + uintptr(i)*unsafe.Sizeof(C.llama_pos(0)))) = C.llama_pos(i)
		*(*C.int32_t)(unsafe.Pointer(uintptr(unsafe.Pointer(batch.n_seq_id)) + uintptr(i)*unsafe.Sizeof(C.int32_t(0)))) = 1
		seqIdPtr := *(**C.llama_seq_id)(unsafe.Pointer(uintptr(unsafe.Pointer(batch.seq_id)) + uintptr(i)*unsafe.Sizeof((*C.llama_seq_id)(nil))))
		*seqIdPtr = 0
		// Mark all tokens as output for embedding
		*(*C.int8_t)(unsafe.Pointer(uintptr(unsafe.Pointer(batch.logits)) + uintptr(i)*unsafe.Sizeof(C.int8_t(0)))) = 1
	}

	// Clear memory
	C.llama_memory_clear(C.llama_get_memory(m.ctx), C.bool(true))

	// Decode to generate embeddings
	ret := C.llama_decode(m.ctx, batch)
	C.llama_batch_free(batch)
	if ret != 0 {
		return nil, fmt.Errorf("llama_decode failed with code %d", ret)
	}

	// Get sequence embeddings (works with pooling type LAST)
	embPtr := C.llama_get_embeddings_seq(m.ctx, 0)
	if embPtr == nil {
		// Fallback to regular embeddings
		embPtr = C.llama_get_embeddings(m.ctx)
		if embPtr == nil {
			return nil, fmt.Errorf("failed to get embeddings")
		}
	}

	// Copy embeddings to Go slice
	embedding := make([]float32, m.nEmbd)
	cEmbeddings := unsafe.Slice((*float32)(unsafe.Pointer(embPtr)), m.nEmbd)
	copy(embedding, cEmbeddings)

	// Normalize to unit vector
	normalize(embedding)

	return embedding, nil
}

// Close frees the model and context
func (m *Model) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctx != nil {
		C.llama_free(m.ctx)
		m.ctx = nil
	}
	if m.model != nil {
		C.llama_model_free(m.model)
		m.model = nil
	}
}

// normalize converts a vector to unit length
func normalize(vec []float32) {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
}
