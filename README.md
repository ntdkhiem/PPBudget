# PPBudget

PPBudget is a powerful, fully-automated personal finance and budgeting application designed for maximum privacy, zero monthly fees, and robust financial tracking.

It securely connects to your bank accounts, imports your transactions automatically every 12 hours, applies a custom rules engine to auto-categorize your spending, and tracks your net worth—all while running on 100% free-tier cloud infrastructure.

---

## 🚀 Features

- **Automated Bank Sync:** Integrates with SimpleFin to automatically pull transactions from your credit cards and bank accounts.
- **Rules-Based Auto-Categorization:** Create powerful rules (e.g., *If description contains 'Uber', set category to 'Travel'*) that execute automatically during every import.
- **Net Worth Tracking:** Automatically snapshots your account balances over time to visualize your net worth trajectory.
- **Budgeting Engine:** Create time-bound budgets and instantly track your spending velocity against your allocated limits.
- **Smart Subscription Tracking:** Detects recurring subscriptions and forecasts your upcoming bills.
- **Linked Transactions:** Handle credit card payments or reimbursements by linking transactions so they don't skew your actual spending/income reports.
- **Email Notifications:** Get daily HTML email summaries of new transactions imported and auto-categorized while you sleep.
- **Automated Backups:** Nightly encrypted `pg_dump` backups of your entire database securely saved to GitHub Actions artifacts.

---

## 🏗️ System Architecture

PPBudget uses a decoupled, serverless-friendly architecture tailored for free-tier cloud deployment:

- **Frontend:** Next.js (React) styled with modern TailwindCSS, deployed to **Vercel**.
- **Backend API:** High-performance Go (Golang) REST API, containerized via Docker and deployed to **Render**.
- **Database:** PostgreSQL (with `pgx` driver), hosted on **Supabase** leveraging their IPv4 Transaction Pooler for zero-downtime serverless connectivity.
- **Cron Jobs:** Scheduled via **GitHub Actions** to trigger bank syncs and database backups without requiring a 24/7 running worker instance.

---

## 🔌 3rd-Party Integrations

1. **SimpleFin ($1.50/mo):** The only paid component of this stack. SimpleFin provides a reliable, read-only token to securely fetch transactions from MX/Plaid without selling your data.
2. **Resend (Free):** Sends daily HTML summary emails. The free tier allows 3,000 emails per month.
3. **Supabase (Free):** Provides a generous 500MB free PostgreSQL database.
4. **Render (Free):** Hosts the Go backend. It spins down after inactivity, but GitHub Actions automatically wakes it up before running syncs.
5. **Vercel (Free):** Edge network hosting for the Next.js frontend.

---

## 💰 Cost Breakdown

The entire architecture is designed to cost nothing, with the exception of the data aggregator.

| Service | Purpose | Cost / Month |
| :--- | :--- | :--- |
| **Vercel** | Frontend Hosting | $0.00 |
| **Render** | Backend Go API | $0.00 |
| **Supabase** | PostgreSQL Database | $0.00 |
| **GitHub** | Cron Jobs & Backup Storage | $0.00 |
| **Resend** | Email Notifications | $0.00 |
| **SimpleFin** | Bank Data Aggregation | $1.50 |
| **Total** | | **$1.50 / mo** |

---

## 💻 Running Locally

### Prerequisites
- Go 1.21+
- Node.js 18+
- PostgreSQL 15+ (or Docker)

### 1. Database Setup
```bash
# Create a local postgres database
psql -c 'CREATE DATABASE ppbudget;'

# The backend will automatically run migrations on startup
```

### 2. Backend (Go)
```bash
# Set your local environment variables
export DATABASE_URL="postgres://postgres:password@localhost:5432/ppbudget?sslmode=disable"
export FRONTEND_URL="http://localhost:3000"

# Run the API server
go run ./cmd/api
```
*The API will start on `http://localhost:8080`.*

### 3. Frontend (Next.js)
```bash
cd frontend

# Install dependencies
npm install

# Start the dev server
npm run dev
```
*The frontend will start on `http://localhost:3000`.*

---

## ☁️ Production Deployment

### 1. Database (Supabase)
- Create a new project on [Supabase](https://supabase.com).
- Navigate to **Database Settings** -> **Connection Pooling**.
- Copy the **Connection Pooler URL** (Port 6543, IPv4).

### 2. Backend (Render)
- Connect your GitHub repo to [Render](https://render.com) and create a new **Web Service**.
- Select the `Dockerfile` in the root directory.
- Add your Environment Variables:
  - `DATABASE_URL`: Your Supabase Pooler URL *(Note: Append `?default_query_exec_mode=exec` to prevent PgBouncer statement caching errors!)*
  - `FRONTEND_URL`: Your Vercel domain (e.g., `https://ppbudget.vercel.app`)
  - `ADMIN_PASSWORD`: A secure password to log into the frontend.
  - `RESEND_API_KEY`: Your Resend API key (optional).
  - `RESEND_FROM_EMAIL`: `onboarding@resend.dev` (optional).

### 3. Frontend (Vercel)
- Import your GitHub repo to [Vercel](https://vercel.com).
- Set the **Root Directory** to `frontend`.
- Add Environment Variable:
  - `NEXT_PUBLIC_API_URL`: Your Render backend URL (e.g., `https://ppbudget.onrender.com/api/v1`)

### 4. Automations (GitHub Actions)
To enable auto-syncs and nightly database backups, add the following **Repository Secrets** to your GitHub repository:
- `BACKEND_API_URL`: Your Render URL (no trailing slash).
- `INGEST_API_KEY`: A random secure string (make sure this matches the `INGEST_API_KEY` on Render!).
- `DATABASE_URL`: Your exact Supabase Connection Pooler string.
- `BACKUP_PASSWORD`: A strong password used to AES-256 encrypt your nightly database ZIP files.

---

## 🔒 Security & Privacy

- Your `ADMIN_PASSWORD` secures the frontend UI via JWT tokens.
- Bank credentials are never seen by the application; SimpleFin only provides read-only transaction feeds.
- The nightly database backups stored on GitHub are encrypted via `openssl aes-256-cbc`. They cannot be read without the `BACKUP_PASSWORD`.
