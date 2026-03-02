// Package files provides the Files API and document ingestion pipeline for antwort.
//
// It enables users to upload files via REST, extract content (via Docling for complex
// formats or passthrough for plain text), chunk the extracted text, embed each chunk,
// and index them in vector stores for search via the file_search tool.
//
// The package defines pluggable interfaces for file storage (FileStore), content
// extraction (ContentExtractor), chunking (Chunker), and vector indexing (VectorIndexer).
// Built-in implementations include filesystem and in-memory file storage, a passthrough
// extractor for text/Markdown/CSV, a Docling HTTP adapter for PDF/DOCX/images, and a
// fixed-size chunker with configurable overlap.
package files
