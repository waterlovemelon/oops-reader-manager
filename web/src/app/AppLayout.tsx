import { BookOutlined, DashboardOutlined, FileTextOutlined, TeamOutlined } from '@ant-design/icons';
import { Layout, Menu, Button } from 'antd';
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { clearAccessToken } from './authStore';

export function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();

  function handleLogout() {
    clearAccessToken();
    navigate('/login');
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Sider theme="light" width={200}>
        <div style={{ height: 56, display: 'grid', placeItems: 'center', fontWeight: 700, fontSize: 16 }}>
          Oops Manager
        </div>
        <Menu
          selectedKeys={[location.pathname]}
          items={[
            { key: '/', icon: <DashboardOutlined />, label: <Link to="/">总览</Link> },
            { key: '/users', icon: <TeamOutlined />, label: <Link to="/users">用户</Link> },
            { key: '/posts', icon: <FileTextOutlined />, label: <Link to="/posts">帖子</Link> },
            { key: '/books', icon: <BookOutlined />, label: <Link to="/books">书籍</Link> },
          ]}
        />
      </Layout.Sider>
      <Layout>
        <Layout.Header style={{ background: '#fff', display: 'flex', justifyContent: 'flex-end', alignItems: 'center', padding: '0 24px' }}>
          <Button onClick={handleLogout}>退出登录</Button>
        </Layout.Header>
        <Layout.Content style={{ padding: 24 }}>
          <Outlet />
        </Layout.Content>
      </Layout>
    </Layout>
  );
}
