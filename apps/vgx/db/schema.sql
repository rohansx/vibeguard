\restrict dbmate

-- Dumped from database version 17.7
-- Dumped by pg_dump version 18.1

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: prevent_audit_modification(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.prevent_audit_modification() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % operations are not permitted', TG_OP;
    RETURN NULL;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_log (
    id bigint NOT NULL,
    org_id text NOT NULL,
    actor_id text,
    actor_type text DEFAULT 'user'::text NOT NULL,
    event_type text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    ip_address inet,
    previous_hash text,
    entry_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: audit_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.audit_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.audit_log_id_seq OWNED BY public.audit_log.id;


--
-- Name: compliance_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.compliance_rules (
    id text NOT NULL,
    version text DEFAULT '1.0'::text NOT NULL,
    regulation text NOT NULL,
    article text,
    paragraph text,
    title text NOT NULL,
    requirement text NOT NULL,
    source_text text NOT NULL,
    source_text_offsets jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_page integer,
    check_type text NOT NULL,
    severity text NOT NULL,
    confidence text DEFAULT 'deterministic'::text NOT NULL,
    languages text[] DEFAULT '{}'::text[] NOT NULL,
    technical_checks jsonb DEFAULT '{}'::jsonb NOT NULL,
    cross_references text[] DEFAULT '{}'::text[] NOT NULL,
    remediation jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_document_id text,
    approved_by text,
    approved_at timestamp with time zone,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.documents (
    id text DEFAULT (gen_random_uuid())::text NOT NULL,
    org_id text NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    file_name text NOT NULL,
    file_path text NOT NULL,
    file_size bigint NOT NULL,
    page_count integer,
    status text DEFAULT 'uploaded'::text NOT NULL,
    extraction_meta jsonb DEFAULT '{}'::jsonb NOT NULL,
    rule_count integer DEFAULT 0 NOT NULL,
    approved_count integer DEFAULT 0 NOT NULL,
    uploaded_by text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: organizations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.organizations (
    id text DEFAULT (gen_random_uuid())::text NOT NULL,
    clerk_org_id text NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    plan text DEFAULT 'free'::text NOT NULL,
    settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: proposed_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.proposed_rules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    document_id text NOT NULL,
    rule_id text NOT NULL,
    version text DEFAULT '1.0'::text NOT NULL,
    regulation text NOT NULL,
    article text,
    paragraph text,
    title text NOT NULL,
    requirement text NOT NULL,
    source_text text NOT NULL,
    source_text_offsets jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_page integer,
    check_type text NOT NULL,
    severity text NOT NULL,
    confidence text DEFAULT 'high'::text NOT NULL,
    languages text[] DEFAULT '{}'::text[] NOT NULL,
    technical_checks jsonb DEFAULT '{}'::jsonb NOT NULL,
    cross_references text[] DEFAULT '{}'::text[] NOT NULL,
    remediation jsonb DEFAULT '{}'::jsonb NOT NULL,
    review_status text DEFAULT 'pending'::text NOT NULL,
    reviewed_by text,
    reviewed_at timestamp with time zone,
    review_notes text,
    original_ai_output jsonb,
    edits_made jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: scan_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scan_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    org_id text NOT NULL,
    initiated_by text,
    repository_url text,
    commit_hash text,
    branch text,
    scan_type text DEFAULT 'full'::text NOT NULL,
    status text DEFAULT 'running'::text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    total_rules_checked integer DEFAULT 0 NOT NULL,
    total_findings integer DEFAULT 0 NOT NULL,
    critical_findings integer DEFAULT 0 NOT NULL,
    high_findings integer DEFAULT 0 NOT NULL,
    medium_findings integer DEFAULT 0 NOT NULL,
    low_findings integer DEFAULT 0 NOT NULL,
    compliance_score numeric(5,2),
    findings jsonb DEFAULT '[]'::jsonb NOT NULL,
    scan_meta jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id text DEFAULT (gen_random_uuid())::text NOT NULL,
    clerk_user_id text NOT NULL,
    org_id text NOT NULL,
    email text NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    role text DEFAULT 'developer'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: audit_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log ALTER COLUMN id SET DEFAULT nextval('public.audit_log_id_seq'::regclass);


--
-- Name: audit_log audit_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log
    ADD CONSTRAINT audit_log_pkey PRIMARY KEY (id);


--
-- Name: compliance_rules compliance_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compliance_rules
    ADD CONSTRAINT compliance_rules_pkey PRIMARY KEY (id);


--
-- Name: documents documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_pkey PRIMARY KEY (id);


--
-- Name: organizations organizations_clerk_org_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_clerk_org_id_key UNIQUE (clerk_org_id);


--
-- Name: organizations organizations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);


--
-- Name: organizations organizations_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.organizations
    ADD CONSTRAINT organizations_slug_key UNIQUE (slug);


--
-- Name: proposed_rules proposed_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proposed_rules
    ADD CONSTRAINT proposed_rules_pkey PRIMARY KEY (id);


--
-- Name: scan_results scan_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scan_results
    ADD CONSTRAINT scan_results_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: users users_clerk_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_clerk_user_id_key UNIQUE (clerk_user_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_audit_log_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_log_created ON public.audit_log USING btree (created_at);


--
-- Name: idx_audit_log_event; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_log_event ON public.audit_log USING btree (event_type);


--
-- Name: idx_audit_log_org; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_log_org ON public.audit_log USING btree (org_id);


--
-- Name: idx_audit_log_resource; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_log_resource ON public.audit_log USING btree (resource_type, resource_id);


--
-- Name: idx_compliance_rules_regulation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_compliance_rules_regulation ON public.compliance_rules USING btree (regulation);


--
-- Name: idx_compliance_rules_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_compliance_rules_status ON public.compliance_rules USING btree (status);


--
-- Name: idx_documents_org; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_documents_org ON public.documents USING btree (org_id);


--
-- Name: idx_documents_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_documents_status ON public.documents USING btree (status);


--
-- Name: idx_proposed_rules_document; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proposed_rules_document ON public.proposed_rules USING btree (document_id);


--
-- Name: idx_proposed_rules_regulation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proposed_rules_regulation ON public.proposed_rules USING btree (regulation);


--
-- Name: idx_proposed_rules_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_proposed_rules_status ON public.proposed_rules USING btree (review_status);


--
-- Name: idx_scan_results_org; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scan_results_org ON public.scan_results USING btree (org_id);


--
-- Name: idx_scan_results_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_scan_results_status ON public.scan_results USING btree (status);


--
-- Name: idx_users_clerk; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_clerk ON public.users USING btree (clerk_user_id);


--
-- Name: idx_users_org; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_org ON public.users USING btree (org_id);


--
-- Name: audit_log audit_log_no_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_log_no_delete BEFORE DELETE ON public.audit_log FOR EACH ROW EXECUTE FUNCTION public.prevent_audit_modification();


--
-- Name: audit_log audit_log_no_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_log_no_update BEFORE UPDATE ON public.audit_log FOR EACH ROW EXECUTE FUNCTION public.prevent_audit_modification();


--
-- Name: audit_log audit_log_actor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log
    ADD CONSTRAINT audit_log_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES public.users(id);


--
-- Name: audit_log audit_log_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log
    ADD CONSTRAINT audit_log_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id);


--
-- Name: compliance_rules compliance_rules_approved_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compliance_rules
    ADD CONSTRAINT compliance_rules_approved_by_fkey FOREIGN KEY (approved_by) REFERENCES public.users(id);


--
-- Name: compliance_rules compliance_rules_source_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.compliance_rules
    ADD CONSTRAINT compliance_rules_source_document_id_fkey FOREIGN KEY (source_document_id) REFERENCES public.documents(id);


--
-- Name: documents documents_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: documents documents_uploaded_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.documents
    ADD CONSTRAINT documents_uploaded_by_fkey FOREIGN KEY (uploaded_by) REFERENCES public.users(id);


--
-- Name: proposed_rules proposed_rules_document_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proposed_rules
    ADD CONSTRAINT proposed_rules_document_id_fkey FOREIGN KEY (document_id) REFERENCES public.documents(id) ON DELETE CASCADE;


--
-- Name: proposed_rules proposed_rules_reviewed_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.proposed_rules
    ADD CONSTRAINT proposed_rules_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES public.users(id);


--
-- Name: scan_results scan_results_initiated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scan_results
    ADD CONSTRAINT scan_results_initiated_by_fkey FOREIGN KEY (initiated_by) REFERENCES public.users(id);


--
-- Name: scan_results scan_results_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scan_results
    ADD CONSTRAINT scan_results_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- Name: users users_org_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_org_id_fkey FOREIGN KEY (org_id) REFERENCES public.organizations(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict dbmate


--
-- Dbmate schema migrations
--

INSERT INTO public.schema_migrations (version) VALUES
    ('20260210000000'),
    ('20260211000000'),
    ('20260212000000');
