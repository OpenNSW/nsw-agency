import { Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from './components/Layout'
import { ConsignmentListScreen } from './screens/ConsignmentListScreen'
import { ConsignmentDetailScreen } from './screens/ConsignmentDetailScreen'

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Navigate to="/consignments" replace />} />
        <Route path="/consignments" element={<ConsignmentListScreen />} />
        <Route path="/consignments/:consignmentId" element={<ConsignmentDetailScreen />} />
      </Route>
    </Routes>
  )
}

export default App
