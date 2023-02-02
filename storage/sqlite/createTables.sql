CREATE TABLE IF NOT EXISTS global(key TEXT, value TEXT, version INTEGER,  UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS ledger(key TEXT, value TEXT, version INTEGER,  UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS blocks(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS blockIndex(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS events(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS transactions(key TEXT, value TEXT, version INTEGER,  UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS collections(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));
CREATE TABLE IF NOT EXISTS transactionResults(key TEXT, value TEXT, version INTEGER, UNIQUE(key,version));

