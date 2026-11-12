/**
 * Shapes returned by the REST API (PostgreSQL schema).
 * @typedef {{id:string,email:string,full_name:string,timezone:string}} User
 * @typedef {{id:string,name:string,icon:?string,color:?string,deleted_at:?string,created_at:string}} Cluster
 * @typedef {{id:string,cluster_id:string,name:string,tracking_type:'duration'|'count',unit_name:?string,deleted_at:?string,created_at:string}} Activity
 * @typedef {{id:string,activity_id:string,log_date:string,start_time:string,end_time:?string,logged_value:?(string|number),entry_status:'NORMAL'|'LATE',notes:?string,created_at:string,updated_at:string}} DailyLog
 * @typedef {{id:string,log_id:string,action_type:'UPDATE'|'DELETE',old_start_time:?string,old_end_time:?string,old_logged_value:?(string|number),changed_at:string}} AuditRow
 */
export {};
