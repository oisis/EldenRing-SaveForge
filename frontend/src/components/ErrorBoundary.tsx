import { Component, ReactNode } from 'react';
import { reportDiagnosticClientError } from '../lib/diagnosticClientErrors';

interface Props { children: ReactNode; }
interface State { hasError: boolean; error: string; }

export class ErrorBoundary extends Component<Props, State> {
    state: State = { hasError: false, error: '' };

    static getDerivedStateFromError(error: Error): State {
        return { hasError: true, error: error.message };
    }

    componentDidCatch(error: Error) {
        reportDiagnosticClientError('render', error);
    }

    render() {
        if (this.state.hasError) {
            return (
                <div className="flex flex-col items-center justify-center h-full p-10">
                    <p className="text-red-500 font-bold">Something went wrong</p>
                    <p className="text-xs text-muted-foreground mt-2">{this.state.error}</p>
                    <button
                        onClick={() => this.setState({ hasError: false, error: '' })}
                        className="mt-4 px-4 py-2 bg-primary text-primary-foreground rounded-md text-xs"
                    >
                        Try Again
                    </button>
                </div>
            );
        }
        return this.props.children;
    }
}
