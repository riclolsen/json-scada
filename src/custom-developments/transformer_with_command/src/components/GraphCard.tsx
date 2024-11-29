import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from './ui/card'
import { RealTimeBarGraph } from './RealTimeBarGraph'

interface GraphCardProps {
  title: string
  description?: string
}

export function GraphCard({ title, description }: GraphCardProps) {
  return (
    <Card className="w-full">
      <CardHeader className="bg-white text-black">
        <CardTitle className="text-xl font-bold">{title}</CardTitle>
        {description && (
          <CardDescription className="text-gray-600">
            {description}
          </CardDescription>
        )}
      </CardHeader>
      <CardContent>
        <RealTimeBarGraph />
      </CardContent>
    </Card>
  )
}
