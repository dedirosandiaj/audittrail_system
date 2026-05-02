-- applications: tenant registry (uPayment, uCuan, uKasir, ...)
CREATE TABLE IF NOT EXISTS applications (
    id         UUID PRIMARY KEY,
    code       VARCHAR(50)  NOT NULL UNIQUE,
    name       VARCHAR(100) NOT NULL DEFAULT '',
    type       VARCHAR(20)  NOT NULL, -- web | mobile | backend
    api_key    VARCHAR(80)  NOT NULL UNIQUE,
    secret_key VARCHAR(160) NOT NULL,
    status     VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_applications_status ON applications (status);
