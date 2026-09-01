package db

// Schema 全部建表 SQL（utf8mb4）。启动时自动执行。
const Schema = `
CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(30) NOT NULL UNIQUE,
  email VARCHAR(254) NOT NULL UNIQUE,
  password_hash VARCHAR(100) NOT NULL,
  role ENUM('new_user','user','admin') NOT NULL DEFAULT 'new_user',
  avatar VARCHAR(255) NOT NULL DEFAULT '',
  bio VARCHAR(160) NOT NULL DEFAULT '',
  expertise VARCHAR(200) NOT NULL DEFAULT '',
  token_version INT NOT NULL DEFAULT 0,
  suspended_until DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS categories (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(50) NOT NULL UNIQUE,
  active TINYINT(1) NOT NULL DEFAULT 1,
  position INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS questions (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  category_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(160) NOT NULL,
  body TEXT NOT NULL,
  tags VARCHAR(250) NOT NULL DEFAULT '',
  views INT UNSIGNED NOT NULL DEFAULT 0,
  status ENUM('open','solved','closed') NOT NULL DEFAULT 'open',
  accepted_answer_id BIGINT UNSIGNED NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  edited_at DATETIME NULL,
  KEY idx_questions_created (created_at),
  KEY idx_questions_category (category_id),
  FULLTEXT KEY ft_questions_title (title, body)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS answers (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  question_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  edited_at DATETIME NULL,
  KEY idx_answers_question (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS articles (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(160) NOT NULL,
  summary VARCHAR(300) NOT NULL DEFAULT '',
  body TEXT NOT NULL,
  tags VARCHAR(250) NOT NULL DEFAULT '',
  views INT UNSIGNED NOT NULL DEFAULT 0,
  published TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  edited_at DATETIME NULL,
  KEY idx_articles_created (created_at),
  FULLTEXT KEY ft_articles_title (title, body)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS forum_boards (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(50) NOT NULL UNIQUE,
  description VARCHAR(200) NOT NULL DEFAULT '',
  position INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS forum_posts (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  board_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(160) NOT NULL,
  body TEXT NOT NULL,
  views INT UNSIGNED NOT NULL DEFAULT 0,
  is_anonymous TINYINT(1) NOT NULL DEFAULT 0,
  is_pinned TINYINT(1) NOT NULL DEFAULT 0,
  is_solved TINYINT(1) NOT NULL DEFAULT 0,
  tags VARCHAR(250) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  edited_at DATETIME NULL,
  KEY idx_forum_posts_created (created_at),
  KEY idx_forum_posts_board (board_id),
  FULLTEXT KEY ft_forum_posts_title (title, body)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS forum_replies (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  post_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_forum_replies_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  question_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  body VARCHAR(500) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_comments_question (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS votes (
  user_id BIGINT UNSIGNED NOT NULL,
  target_type ENUM('question','answer','forum_post','forum_reply','article') NOT NULL,
  target_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS bookmarks (
  user_id BIGINT UNSIGNED NOT NULL,
  question_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS question_follows (
  user_id BIGINT UNSIGNED NOT NULL,
  question_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_follows (
  follower_id BIGINT UNSIGNED NOT NULL,
  followed_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (follower_id, followed_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notifications (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  type VARCHAR(20) NOT NULL,
  message VARCHAR(500) NOT NULL,
  question_id BIGINT UNSIGNED NULL,
  is_read TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_notifications_user (user_id, is_read, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS job_entries (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  last_editor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  company VARCHAR(100) NOT NULL,
  industry VARCHAR(200) NOT NULL DEFAULT '',
  city VARCHAR(100) NOT NULL DEFAULT '',
  apply_link VARCHAR(1000) NOT NULL DEFAULT '',
  referral_code VARCHAR(50) NOT NULL DEFAULT '',
  verified_at VARCHAR(10) NOT NULL DEFAULT '',
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  is_pinned TINYINT(1) NOT NULL DEFAULT 0,
  edit_reason VARCHAR(200) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_job_entries_updated (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS job_entry_versions (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  job_id BIGINT UNSIGNED NOT NULL,
  editor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  reason VARCHAR(200) NOT NULL DEFAULT '',
  company VARCHAR(100) NOT NULL DEFAULT '',
  industry VARCHAR(200) NOT NULL DEFAULT '',
  city VARCHAR(100) NOT NULL DEFAULT '',
  apply_link VARCHAR(1000) NOT NULL DEFAULT '',
  referral_code VARCHAR(50) NOT NULL DEFAULT '',
  verified_at VARCHAR(10) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_job_versions_job (job_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS job_reviews (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  job_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  body VARCHAR(500) NOT NULL,
  is_anonymous TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_job_reviews_job (job_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS article_comments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  article_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  body VARCHAR(500) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_article_comments_article (article_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS uploads_meta (
  filename VARCHAR(255) PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS admin_audit (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  actor_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(50) NOT NULL,
  target_type VARCHAR(30) NOT NULL DEFAULT '',
  target_id BIGINT UNSIGNED NULL,
  details VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_admin_audit_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`

// Seed 默认数据：分类与论坛板块。仅在对应表为空时插入。
var Seed = []string{
	`INSERT INTO categories (name, active, position) VALUES
	 ('光学设计问答', 1, 0),
	 ('光学考研问答', 1, 1),
	 ('Zemax 使用', 1, 2),
	 ('光学就业', 1, 3),
	 ('知识文章', 1, 4),
	 ('学习资料', 1, 5)
	 ON DUPLICATE KEY UPDATE name=name`,
	`INSERT INTO forum_boards (name, description, position) VALUES
	 ('综合讨论', '光学相关的一切讨论', 0),
	 ('学术前沿', '论文、会议与行业动态', 1),
	 ('资源分享', '学习资料、工具与经验', 2)
	 ON DUPLICATE KEY UPDATE name=name`,
}
