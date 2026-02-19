import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import SLOBurnRate from './SLOBurnRate'

describe('SLOBurnRate', () => {
  it('renders SLO target', () => {
    render(<SLOBurnRate target={0.999} totalExecutions={1000} failedExecutions={0} />)
    expect(screen.getByText('SLO: 99.9%')).toBeInTheDocument()
  })

  it('shows healthy status when SLO is met', () => {
    render(<SLOBurnRate target={0.99} totalExecutions={1000} failedExecutions={5} />)
    expect(screen.getByText('● Healthy')).toBeInTheDocument()
  })

  it('shows at-risk when SLO is breached', () => {
    render(<SLOBurnRate target={0.999} totalExecutions={1000} failedExecutions={10} />)
    expect(screen.getByText('● At Risk')).toBeInTheDocument()
  })

  it('calculates SLO percentage correctly', () => {
    render(<SLOBurnRate target={0.99} totalExecutions={100} failedExecutions={2} />)
    expect(screen.getByText('98.00%')).toBeInTheDocument()
  })

  it('shows 100% when no failures', () => {
    render(<SLOBurnRate target={0.999} totalExecutions={500} failedExecutions={0} />)
    expect(screen.getByText('100.00%')).toBeInTheDocument()
  })

  it('handles zero executions gracefully', () => {
    render(<SLOBurnRate target={0.999} totalExecutions={0} failedExecutions={0} />)
    expect(screen.getByText('100.00%')).toBeInTheDocument()
    expect(screen.getByText('● Healthy')).toBeInTheDocument()
  })

  it('displays execution count and window label', () => {
    render(<SLOBurnRate target={0.99} totalExecutions={5000} failedExecutions={10} windowLabel="7d" />)
    expect(screen.getByText(/5,000 executions/)).toBeInTheDocument()
    expect(screen.getByText(/7d/)).toBeInTheDocument()
  })

  it('shows error budget remaining', () => {
    // 99% SLO with 1000 total = 10 error budget, 3 failures = 7 remaining
    render(<SLOBurnRate target={0.99} totalExecutions={1000} failedExecutions={3} />)
    expect(screen.getByText(/7 \/ 10 remaining/)).toBeInTheDocument()
  })

  it('shows burn rate', () => {
    render(<SLOBurnRate target={0.99} totalExecutions={1000} failedExecutions={5} />)
    expect(screen.getByText('Burn Rate')).toBeInTheDocument()
  })

  it('uses default window label', () => {
    render(<SLOBurnRate target={0.99} totalExecutions={100} failedExecutions={1} />)
    expect(screen.getByText(/30-day/)).toBeInTheDocument()
  })
})
