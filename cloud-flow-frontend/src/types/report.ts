export interface Report {
  id: string;
  title: string;
  type: string;
  status: string;
  createdAt: string;
  generatedAt?: string;
  downloadUrl?: string;
  size?: string;
}

export interface ReportDetail extends Report {
  content?: string;
  summary?: string;
  charts?: ReportChart[];
}

export interface ReportChart {
  id: string;
  title: string;
  type: string;
  data: Record<string, unknown>[];
}

export interface ReportSearchParams {
  page?: number;
  pageSize?: number;
  type?: string;
  status?: string;
  startTime?: string;
  endTime?: string;
}

export interface GenerateReportRequest {
  type: string;
  title: string;
  startTime: string;
  endTime: string;
  params?: {
    format?: string;
    includeCharts?: boolean;
    [key: string]: string | boolean | number | undefined;
  };
}