import { useState } from 'react'
import WorkflowBuilder, { Workflow } from '../components/workflow/WorkflowBuilder'

export default function Workflows() {
  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [activeWorkflow, setActiveWorkflow] = useState<Workflow | null>(null)
  const [isEditing, setIsEditing] = useState(false)

  const createNewWorkflow = () => {
    const newWorkflow: Workflow = {
      id: `wf-${Date.now()}`,
      name: 'New Workflow',
      description: '',
      nodes: [],
      edges: [],
    }
    setActiveWorkflow(newWorkflow)
    setIsEditing(true)
  }

  const saveWorkflow = (workflow: Workflow) => {
    setWorkflows((prev) => {
      const existing = prev.findIndex((w) => w.id === workflow.id)
      if (existing >= 0) {
        const updated = [...prev]
        updated[existing] = workflow
        return updated
      }
      return [...prev, workflow]
    })
    setIsEditing(false)
    setActiveWorkflow(null)
  }

  const deleteWorkflow = (id: string) => {
    if (confirm('Are you sure you want to delete this workflow?')) {
      setWorkflows((prev) => prev.filter((w) => w.id !== id))
    }
  }

  if (isEditing && activeWorkflow) {
    return (
      <div className="h-[calc(100vh-10rem)]">
        <div className="flex justify-between items-center mb-4">
          <div className="flex items-center gap-4">
            <button
              onClick={() => {
                setIsEditing(false)
                setActiveWorkflow(null)
              }}
              className="text-gray-600 hover:text-gray-800"
            >
              ← Back
            </button>
            <input
              type="text"
              value={activeWorkflow.name}
              onChange={(e) =>
                setActiveWorkflow({ ...activeWorkflow, name: e.target.value })
              }
              className="text-2xl font-bold text-gray-900 bg-transparent border-b border-transparent hover:border-gray-300 focus:border-indigo-500 focus:outline-none"
            />
          </div>
          <button
            onClick={() => saveWorkflow(activeWorkflow)}
            className="px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700"
          >
            Save Workflow
          </button>
        </div>
        <div className="bg-white rounded-lg shadow h-full">
          <WorkflowBuilder
            workflow={activeWorkflow}
            onChange={setActiveWorkflow}
          />
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold text-gray-900">Workflows</h2>
        <button
          onClick={createNewWorkflow}
          className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700"
        >
          Create Workflow
        </button>
      </div>

      <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <span className="text-yellow-400">⚠️</span>
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-yellow-800">
              Experimental Feature
            </h3>
            <p className="mt-1 text-sm text-yellow-700">
              Visual Workflow Builder is under active development. Workflows are stored in browser memory only.
            </p>
          </div>
        </div>
      </div>

      <div className="bg-white shadow overflow-hidden sm:rounded-md">
        {workflows.length === 0 ? (
          <div className="text-center py-12">
            <div className="text-6xl mb-4">🎨</div>
            <p className="text-gray-500 mb-4">No workflows yet</p>
            <p className="text-sm text-gray-400 mb-6">
              Create visual DAG-based workflows to orchestrate multiple jobs
            </p>
            <button
              onClick={createNewWorkflow}
              className="text-indigo-600 hover:text-indigo-800"
            >
              Create your first workflow
            </button>
          </div>
        ) : (
          <ul className="divide-y divide-gray-200">
            {workflows.map((workflow) => (
              <li key={workflow.id}>
                <div className="px-4 py-4 flex items-center sm:px-6">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="text-lg">🎨</span>
                      <button
                        onClick={() => {
                          setActiveWorkflow(workflow)
                          setIsEditing(true)
                        }}
                        className="font-medium text-indigo-600 hover:text-indigo-800"
                      >
                        {workflow.name}
                      </button>
                    </div>
                    <div className="mt-1 text-sm text-gray-500">
                      {workflow.nodes.length} nodes · {workflow.edges.length} connections
                    </div>
                    {workflow.description && (
                      <div className="mt-1 text-sm text-gray-400">
                        {workflow.description}
                      </div>
                    )}
                  </div>
                  <div className="ml-5 flex-shrink-0 flex space-x-2">
                    <button
                      onClick={() => {
                        setActiveWorkflow(workflow)
                        setIsEditing(true)
                      }}
                      className="px-3 py-1 text-sm text-indigo-600 hover:text-indigo-800 border border-indigo-300 rounded hover:bg-indigo-50"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => deleteWorkflow(workflow.id)}
                      className="px-3 py-1 text-sm text-red-600 hover:text-red-800 border border-red-300 rounded hover:bg-red-50"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
