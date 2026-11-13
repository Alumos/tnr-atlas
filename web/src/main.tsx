import React from 'react'
import ReactDOM from 'react-dom/client'
import { ThemeProvider } from 'next-themes'
import App from './App'
import { lazy, Suspense } from 'react'
import './index.css'
const Admin = lazy(() => import('./Admin'))
ReactDOM.createRoot(document.getElementById('root')!).render(<React.StrictMode><ThemeProvider attribute="class" defaultTheme="light" enableSystem>{location.pathname === '/admin' ? <Suspense fallback={<p className="p-8">正在加载后台…</p>}><Admin /></Suspense> : <App />}</ThemeProvider></React.StrictMode>)
