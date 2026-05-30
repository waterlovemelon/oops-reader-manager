import { UploadOutlined } from '@ant-design/icons';
import { Button, Card, Space, Table, Tag, Upload, message } from 'antd';
import type { UploadProps } from 'antd';
import { useState } from 'react';
import { CatalogBook, uploadBook } from '../api/catalog';

export function BooksPage() {
  const [books, setBooks] = useState<CatalogBook[]>([]);
  const [uploading, setUploading] = useState(false);

  const props: UploadProps = {
    accept: '.epub,.txt',
    showUploadList: false,
    beforeUpload: async (file) => {
      setUploading(true);
      try {
        const book = await uploadBook(file);
        setBooks((current) => [book, ...current]);
        message.success('上传成功，书籍已进入草稿');
      } catch (error) {
        message.error(error instanceof Error ? error.message : '上传失败');
      } finally {
        setUploading(false);
      }
      return false;
    },
  };

  return (
    <Card
      title="在线书籍"
      extra={
        <Upload {...props}>
          <Button icon={<UploadOutlined />} loading={uploading}>上传 EPUB/TXT</Button>
        </Upload>
      }
    >
      <Table
        rowKey="id"
        dataSource={books}
        columns={[
          { title: '书名', dataIndex: 'title' },
          { title: '作者', dataIndex: 'author' },
          { title: '格式', dataIndex: 'format', render: (value: string) => <Tag>{value}</Tag> },
          { title: '章节', dataIndex: 'chapter_count' },
          { title: '状态', dataIndex: 'status', render: (value: string) => <Tag color={value === 'draft' ? 'gold' : 'green'}>{value}</Tag> },
          {
            title: '操作',
            render: () => <Space><Button size="small">编辑</Button><Button size="small">发布</Button></Space>,
          },
        ]}
      />
    </Card>
  );
}
