DROP TABLE IF EXISTS admin_messages CASCADE;
DROP TABLE IF EXISTS registration_requests CASCADE;
DROP TABLE IF EXISTS tokens CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS admins CASCADE;

-- Таблица для хранения пользователей
CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       first_name VARCHAR(255) NOT NULL,
                       last_name VARCHAR(255) NOT NULL,
                       birth_date DATE NOT NULL,
                       status VARCHAR(50) NOT NULL CHECK (status IN ('student', 'employee', 'graduate')),
                       phone_number VARCHAR(20) NOT NULL UNIQUE,
                       photo_path VARCHAR(255), -- Путь к фото (заглушка для будущего использования)
                       expires_at TIMESTAMP WITH TIME ZONE NOT NULL, -- Срок действия подписки
                       created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Таблица для хранения заявок на регистрацию
CREATE TABLE registration_requests (
                                       id SERIAL PRIMARY KEY,
                                       user_id INT, -- Ссылка на пользователя (заполняется после одобрения)
                                       telegram_user_id BIGINT, -- Telegram ID пользователя
                                       first_name VARCHAR(255) NOT NULL,
                                       last_name VARCHAR(255) NOT NULL,
                                       birth_date DATE NOT NULL,
                                       user_status VARCHAR(50) NOT NULL CHECK (user_status IN ('student', 'employee', 'graduate')),
                                       document_path VARCHAR(255), -- Путь к документу (например, doc_files/xxx.jpg)
                                       phone_number VARCHAR(20) NOT NULL,
                                       status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
                                       created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                                       updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                                       CONSTRAINT fk_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Таблица для хранения кодов входа
CREATE TABLE tokens (
                        id SERIAL PRIMARY KEY,
                        user_id INT NOT NULL,
                        token VARCHAR(255) NOT NULL UNIQUE, -- Шестизначный код (в будущем JWT)
                        phone_number VARCHAR(20) NOT NULL,
                        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                        expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
                        CONSTRAINT fk_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Таблица для хранения администраторов
CREATE TABLE admins (
                        id SERIAL PRIMARY KEY,
                        chat_id BIGINT NOT NULL UNIQUE, -- Telegram Chat ID админа
                        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create admin_messages table
CREATE TABLE admin_messages (
                                id SERIAL PRIMARY KEY,
                                telegram_user_id BIGINT NOT NULL,
                                first_name VARCHAR(255) NOT NULL,
                                last_name VARCHAR(255) NOT NULL,
                                message TEXT NOT NULL,
                                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индексы для оптимизации
CREATE INDEX idx_registration_requests_status ON registration_requests(status);
CREATE INDEX idx_tokens_user_id ON tokens(user_id);
CREATE INDEX idx_admins_chat_id ON admins(chat_id);

ALTER TABLE registration_requests DROP CONSTRAINT registration_requests_status_check;
ALTER TABLE registration_requests ADD CONSTRAINT registration_requests_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'on_hold'));

ALTER TABLE registration_requests DROP CONSTRAINT registration_requests_status_check;
ALTER TABLE registration_requests ADD CONSTRAINT registration_requests_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'on_hold', 'needs_revision'));

ALTER TABLE registration_requests ADD COLUMN rejection_reason TEXT;