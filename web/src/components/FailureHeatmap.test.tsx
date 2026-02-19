import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import FailureHeatmap from './FailureHeatmap'
import type { Execution } from '../types'

function makeExecution(overrides: Partial<Execution> = {}): Execution {
  return {
    id: 'exec-1',
    job_id: 'job-1',
    job_name: 'Test Job',
    scheduled_time: '2026-01-15T09:00:00Z',
    started_at: '2026-01-15T09:00:01Z',
    completed_at: '2026-01-15T09:00:05Z',
    status: 'success',
    attempts: 1,
    duration: 4000,
    ...overrides,
  }
}

describe('FailureHeatmap', () => {
  it('renders with empty executions', () => {
    render(<FailureHeatmap executions={[]} />)
    expect(screen.getByText('Failure Heatmap')).toBeInTheDocument()
  })

  it('renders day labels', () => {
    render(<FailureHeatmap executions={[]} />)
    expect(screen.getByText('Mon')).toBeInTheDocument()
    expect(screen.getByText('Fri')).toBeInTheDocument()
    expect(screen.getByText('Sun')).toBeInTheDocument()
  })

  it('renders legend', () => {
    render(<FailureHeatmap executions={[]} />)
    expect(screen.getByText('Less')).toBeInTheDocument()
    expect(screen.getByText('More failures')).toBeInTheDocument()
  })

  it('renders with executions data', () => {
    const executions: Execution[] = [
      makeExecution({ status: 'success', started_at: '2026-01-13T10:00:00Z' }),
      makeExecution({ id: 'e2', status: 'failed', started_at: '2026-01-13T10:05:00Z' }),
      makeExecution({ id: 'e3', status: 'success', started_at: '2026-01-14T14:00:00Z' }),
    ]

    const { container } = render(<FailureHeatmap executions={executions} />)
    // Should render 7 days × 24 hours = 168 cells + header row
    const cells = container.querySelectorAll('[title]')
    expect(cells.length).toBe(168)
  })

  it('handles mixed statuses correctly', () => {
    const executions: Execution[] = [
      makeExecution({ status: 'success' }),
      makeExecution({ id: 'e2', status: 'failed' }),
      makeExecution({ id: 'e3', status: 'running' }),
      makeExecution({ id: 'e4', status: 'skipped' }),
    ]

    // Should not throw
    const { container } = render(<FailureHeatmap executions={executions} />)
    expect(container).toBeTruthy()
  })
})
