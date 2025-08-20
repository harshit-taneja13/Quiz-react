import { useState } from 'react'
import './App.css'

function App() {
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [linkedin, setLinkedin] = useState('')
  const [status, setStatus] = useState({ loading: false, message: '' })

  const apiBase = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  async function handleSubmit(e) {
    e.preventDefault()
    setStatus({ loading: true, message: '' })
    try {
      const res = await fetch(`${apiBase}/api/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, phone, linkedin })
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data?.message || 'Request failed')
      setStatus({ loading: false, message: 'Saved! Your submission was recorded.' })
      setName('')
      setPhone('')
      setLinkedin('')
    } catch (err) {
      setStatus({ loading: false, message: `Error: ${err.message}` })
    }
  }

  return (
    <div className="card" style={{ maxWidth: 440, margin: '40px auto', textAlign: 'left' }}>
      <h2 style={{ textAlign: 'center' }}>React Quiz - Login</h2>
      <form onSubmit={handleSubmit}>
        <div style={{ marginBottom: 12 }}>
          <label htmlFor="name">Name</label>
          <input
            id="name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Your full name"
            required
            style={{ width: '100%' }}
          />
        </div>
        <div style={{ marginBottom: 12 }}>
          <label htmlFor="phone">Phone</label>
          <input
            id="phone"
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder="Phone number"
            required
            style={{ width: '100%' }}
          />
        </div>
        <div style={{ marginBottom: 16 }}>
          <label htmlFor="linkedin">LinkedIn (optional)</label>
          <input
            id="linkedin"
            type="url"
            value={linkedin}
            onChange={(e) => setLinkedin(e.target.value)}
            placeholder="https://www.linkedin.com/in/username"
            style={{ width: '100%' }}
          />
        </div>
        <button type="submit" disabled={status.loading}>
          {status.loading ? 'Submitting...' : 'Submit'}
        </button>
      </form>
      {status.message && (
        <p style={{ marginTop: 16 }}>{status.message}</p>
      )}
    </div>
  )
}

export default App
