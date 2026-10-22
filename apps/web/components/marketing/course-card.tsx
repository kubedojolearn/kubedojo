import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { PlannedCourse } from "@/lib/curriculum";

export function CourseCard({
  course,
  index,
  showLessons = false,
}: {
  course: PlannedCourse;
  index: number;
  showLessons?: boolean;
}) {
  return (
    <Card className="h-full" aria-labelledby={`course-${course.slug}`}>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <span className="font-mono text-xs text-muted-foreground">Course {index + 1}</span>
          <Badge variant="outline" className="capitalize">
            {course.difficulty}
          </Badge>
        </div>
        <CardTitle id={`course-${course.slug}`} className="text-lg">
          {course.title}
        </CardTitle>
        <CardDescription>{course.summary}</CardDescription>
      </CardHeader>
      <CardContent className="mt-auto">
        {showLessons ? (
          <ol className="grid gap-1.5 text-sm">
            {course.lessons.map((lesson, i) => (
              <li key={lesson.slug} className="flex gap-2">
                <span className="w-5 shrink-0 text-right font-mono text-xs leading-5 text-muted-foreground">
                  {i + 1}.
                </span>
                <span>{lesson.title}</span>
              </li>
            ))}
          </ol>
        ) : (
          <p className="text-sm text-muted-foreground">{course.lessons.length} lessons</p>
        )}
      </CardContent>
    </Card>
  );
}
