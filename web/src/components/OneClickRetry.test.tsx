import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import OneClickRetry from './OneClickRetry'
import type { Execution } from '../types'

// Mock the API client
vi.mock('../api/client', () => ({
  triggerJob: vi.fn(),
  replayExecution: vi.fn(),
}))

import { triggerJob, replayExecution } from '../api/client'

const mockTriggerJob = vi.mocked(triggerJob)
const mockReplayExecution = vi.mocked(replayExecution)

function makeExecution(overrides: Partial<Execution> = {}): Execution {
  return {
    id: 'exec-1',
    job_id: 'job-1',
    job_name: 'Test Job',
    scheduled_time: '2026-01-15T09:00:00Z',
    started_at: '2026-01-15T09:00:01Z',
    completed_at: '2026-01-15T09:00:05Z',
    status: 'failed',
    attempts: 1,
    error: 'Connection refused',
    duration: 4000,
    ...overrides,
  }
}

describe('OneClickRetry', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('renders nothing for successful executions', () => {
    const { container } = render(
      <OneClickRetry execution={makeExecution({ status: 'success' })} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders nothing for running executions', () => {
    const { container } = render(
      <OneClickRetry execution={makeExecution({ status: 'running' })} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders retry button for failed executions', () => {
    render(<OneClickRetry execution={makeExecution()} />)
    expect(screen.getByText('Retry')).toBeInTheDocument()
  })

  it('calls replayExecution on click', async () => {
    const retriedExec = makeExecution({ id: 'exec-2', status: 'running' })
    mockReplayExecution.mockResolvedValue(retriedExec)

    render(<OneClickRetry execution={makeExecution()} />)
    fireEvent.click(screen.getByText('Retry'))

    await waitFor(() => {
      expect(mockReplayExecution).toHaveBeenCalledWith('job-1', 'exec-1')
    })

    await waitFor(() => {
      expect(screen.getByText('Retried')).toBeInTheDocument()
    })
  })

  it('falls back to triggerJob when replay fails', async () => {
    mockReplayExecution.mockRejectedValue(new Error('replay not supported'))
    const retriedExec = makeExecution({ id: 'exec-3', status: 'running' })
    mockTriggerJob.mockResolvedValue(retriedExec)

    render(<OneClickRetry execution={makeExecution()} />)
    fireEvent.click(screen.getByText('Retry'))

    await waitFor(() => {
      expect(mockTriggerJob).toHaveBeenCalledWith('job-1')
    })

    await waitFor(() => {
      expect(screen.getByText('Retried')).toBeInTheDocument()
    })
  })

  it('shows error when both retry methods fail', async () => {
    mockReplayExecution.mockRejectedValue(new Error('replay failed'))
    mockTriggerJob.mockRejectedValue(new Error('trigger also failed'))

    render(<OneClickRetry execution={makeExecution()} />)
    fireEvent.click(screen.getByText('Retry'))

    await waitFor(() => {
      expect(screen.getByText('trigger also failed')).toBeInTheDocument()
    })
  })

  it('calls onRetryComplete callback', async () => {
    const retriedExec = makeExecution({ id: 'exec-new', status: 'running' })
    mockReplayExecution.mockResolvedValue(retriedExec)
    const callback = vi.fn()

    render(<OneClickRetry execution={makeExecution()} onRetryComplete={callback} />)
    fireEvent.click(screen.getByText('Retry'))

    await waitFor(() => {
      expect(callback).toHaveBeenCalledWith(retriedExec)
    })
  })

  it('disables button while loading', async () => {
    // Make the promise never resolve to keep loading state
    mockReplayExecution.mockReturnValue(new Promise(() => {}))

    render(<OneClickRetry execution={makeExecution()} />)
    fireEvent.click(screen.getByText('Retry'))

    await waitFor(() => {
      const button = screen.getByRole('button')
      expect(button).toBeDisabled()
    })
  })
})
