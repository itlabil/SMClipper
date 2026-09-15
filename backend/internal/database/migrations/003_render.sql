CREATE TABLE clip_render_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_candidate_id UUID NOT NULL UNIQUE REFERENCES clip_candidates(id) ON DELETE CASCADE,
    layout_type TEXT NOT NULL DEFAULT 'single_crop', -- 'single_crop' | 'split_top_bottom'
    aspect_ratio TEXT NOT NULL DEFAULT '9:16',
    crop_regions_json JSONB,
    -- contoh isi crop_regions_json:
    -- single_crop: [{"x":100,"y":0,"w":600,"h":1080}]
    -- split_top_bottom: [{"speaker":"A","x":..,"y":..,"w":..,"h":..}, {"speaker":"B",...}]
    subtitle_enabled BOOLEAN NOT NULL DEFAULT true,
    subtitle_style_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE rendered_clips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_candidate_id UUID NOT NULL REFERENCES clip_candidates(id) ON DELETE CASCADE,
    job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    output_path TEXT,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, done, failed
    file_size BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_render_configs_clip_id ON clip_render_configs(clip_candidate_id);
CREATE INDEX idx_rendered_clips_clip_id ON rendered_clips(clip_candidate_id);