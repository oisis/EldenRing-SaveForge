import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {ErrorBoundary} from './components/ErrorBoundary'
import {SafetyModeProvider} from './state/safetyMode'
import {FavoritesProvider} from './state/favorites'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ErrorBoundary>
            <SafetyModeProvider>
                <FavoritesProvider>
                    <App/>
                </FavoritesProvider>
            </SafetyModeProvider>
        </ErrorBoundary>
    </React.StrictMode>
)
