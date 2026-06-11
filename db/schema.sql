create extension if not exists pgcrypto;

create table if not exists tasks (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  title text not null,
  description text not null default '',
  status text not null check (status in ('todo', 'in_progress', 'completed')),
  priority text not null check (priority in ('low', 'medium', 'high')),
  due_date timestamptz not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists idx_tasks_user_status_created_at on tasks (user_id, status, created_at desc);
create index if not exists idx_tasks_user_due_date on tasks (user_id, due_date asc);
create index if not exists idx_tasks_title_search on tasks (lower(title));

create table if not exists task_attachments (
  id uuid primary key default gen_random_uuid(),
  task_id uuid not null references tasks (id) on delete cascade,
  user_id text not null,
  file_name text not null,
  mime_type text not null,
  size_bytes bigint not null check (size_bytes > 0),
  storage_path text not null unique,
  created_at timestamptz not null default now()
);

create index if not exists idx_task_attachments_task_id on task_attachments (task_id);

create table if not exists task_activity (
  id uuid primary key default gen_random_uuid(),
  task_id uuid not null references tasks (id) on delete cascade,
  actor_id text not null,
  actor_email text not null default '',
  action text not null,
  changes jsonb not null default '{}',
  created_at timestamptz not null default now()
);

create index if not exists idx_task_activity_task_created_at on task_activity (task_id, created_at desc);
