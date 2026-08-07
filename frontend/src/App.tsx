import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom'
import HomePage from './features/home/Page'
import ShoppingListPage from './features/shopping-list/Page'
import ImageProcessingPage from './features/image-processing/Page'

// Phase 1 shell: nav + routing only. Auth-gating, layout polish, etc. are
// phase 2 once there's something worth protecting.
export default function App() {
  return (
    <BrowserRouter>
      <nav>
        <NavLink to="/">Home</NavLink>{' | '}
        <NavLink to="/shopping-list">Shopping List</NavLink>{' | '}
        <NavLink to="/image-processing">Image Processing</NavLink>
      </nav>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/shopping-list" element={<ShoppingListPage />} />
        <Route path="/image-processing" element={<ImageProcessingPage />} />
      </Routes>
    </BrowserRouter>
  )
}
