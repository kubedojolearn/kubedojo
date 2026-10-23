import { forwardToApi } from "@/lib/api/bff";

type Context = RouteContext<"/api/bff/[...path]">;

async function handle(request: Request, { params }: Context) {
  const { path } = await params;
  return forwardToApi(request, path);
}

export { handle as GET, handle as POST, handle as PUT, handle as PATCH, handle as DELETE };
