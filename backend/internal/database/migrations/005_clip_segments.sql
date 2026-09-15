CREATE TABLE clip_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_candidate_id UUID NOT NULL REFERENCES clip_candidates(id) ON DELETE CASCADE,
    start_ms INTEGER NOT NULL, -- relatif terhadap awal clip (0 = detik pertama clip)
    end_ms INTEGER NOT NULL,
    layout_type TEXT NOT NULL DEFAULT 'single_crop',
    crop_regions_json JSONB NOT NULL DEFAULT '[]',
    order_index INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_clip_segments_clip_id ON clip_segments(clip_candidate_id);