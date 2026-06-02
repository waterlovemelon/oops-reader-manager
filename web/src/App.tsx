import { Navigate, Route, BrowserRouter, Routes } from 'react-router-dom';
import { AppLayout } from './app/AppLayout';
import { getAccessToken } from './app/authStore';
import { DashboardPage } from './pages/DashboardPage';
import { LoginPage } from './pages/LoginPage';
import { BooksPage } from './pages/BooksPage';
import { UsersPage } from './pages/UsersPage';
import { PostsPage } from './pages/PostsPage';
import { AuditPage } from './pages/AuditPage';
import { RecommendedBooksPage } from './pages/RecommendedBooksPage';

function RequireAuth({ children }: { children: JSX.Element }) {
  if (!getAccessToken()) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<RequireAuth><AppLayout /></RequireAuth>}>
          <Route index element={<DashboardPage />} />
          <Route path="books" element={<BooksPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="posts" element={<PostsPage />} />
          <Route path="recommendations" element={<RecommendedBooksPage />} />
          <Route path="audit" element={<AuditPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
