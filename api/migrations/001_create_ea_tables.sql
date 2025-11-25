-- 扩展users表
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance DECIMAL(15, 2) DEFAULT 10000.0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS equity DECIMAL(15, 2) DEFAULT 10000.0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS margin DECIMAL(15, 2) DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS free_margin DECIMAL(15, 2) DEFAULT 10000.0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS margin_level DECIMAL(5, 2) DEFAULT 0;

-- 创建eas表
CREATE TABLE IF NOT EXISTS eas (
    ea_id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    ea_name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    strategy VARCHAR(50) NOT NULL,
    risk_percent DECIMAL(5, 2) DEFAULT 2.0,
    max_positions INT DEFAULT 3,
    enabled BOOLEAN DEFAULT true,
    indicator_params JSONB DEFAULT '{"length": 8, "deviation": 1, "money_risk": 1.0, "signal": 1, "line": 1}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_eas_user_id ON eas(user_id);
CREATE INDEX IF NOT EXISTS idx_eas_enabled ON eas(enabled);

-- 创建positions表
CREATE TABLE IF NOT EXISTS positions (
    position_id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    ea_id UUID REFERENCES eas(ea_id) ON DELETE SET NULL,
    symbol VARCHAR(20) NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('BUY', 'SELL')),
    lots DECIMAL(10, 2) NOT NULL,
    open_price DECIMAL(15, 5) NOT NULL,
    stop_loss DECIMAL(15, 5),
    take_profit DECIMAL(15, 5),
    open_time TIMESTAMP NOT NULL,
    close_time TIMESTAMP,
    close_price DECIMAL(15, 5),
    profit DECIMAL(15, 2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_positions_user_id ON positions(user_id);
CREATE INDEX IF NOT EXISTS idx_positions_ea_id ON positions(ea_id);
CREATE INDEX IF NOT EXISTS idx_positions_status ON positions(status);
CREATE INDEX IF NOT EXISTS idx_positions_open_time ON positions(open_time);

COMMENT ON TABLE eas IS 'EA配置表';
COMMENT ON TABLE positions IS '持仓记录表';
